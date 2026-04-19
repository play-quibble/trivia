// Package realtime manages WebSocket connections and in-process room state.
//
// Architecture (single-replica phase):
//   - One Hub per process, holding all active rooms keyed by game code.
//   - Each room holds the set of connected clients for one game.
//   - Scale-out path: replace in-process broadcast with Redis Pub/Sub fan-out.
//     All call sites (Broadcast, JoinRoom, LeaveRoom) stay the same — only the
//     internals of broadcast() change.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

// MessageType identifies the kind of realtime event being sent or received.
// Using typed string constants (rather than plain strings) means the compiler
// catches typos at build time instead of at runtime.
type MessageType string

const (
	// Server → client events
	MsgQuestionRevealed MessageType = "question_revealed"
	MsgTimerTick        MessageType = "timer_tick"
	MsgAnswerAccepted   MessageType = "answer_accepted"
	MsgScoreboardUpdate MessageType = "scoreboard_update"
	MsgGameEnded        MessageType = "game_ended"

	// Client → server events (sent by the host)
	MsgStartGame       MessageType = "start_game"
	MsgAdvanceQuestion MessageType = "advance_question"
	MsgEndGame         MessageType = "end_game"

	// Client → server events (sent by players)
	MsgJoin         MessageType = "join"
	MsgSubmitAnswer MessageType = "submit_answer"
)

// Message is the wire format for all WebSocket messages — both inbound and
// outbound. Payload is json.RawMessage (raw bytes) so it can hold any
// JSON structure without needing a separate type per message kind.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"` // omitempty: omit from JSON when nil
}

// client represents a single connected WebSocket peer (host or player).
// send is a buffered channel — think of it as an async queue. The writer
// goroutine drains it and writes to the wire. Using a channel means the
// broadcast loop is never blocked by a single slow client.
type client struct {
	conn *websocket.Conn
	send chan Message // buffered; capacity 64 messages
}

// room holds all clients connected to a single game.
// sync.RWMutex is a read-write lock: multiple goroutines can hold a read
// lock simultaneously, but a write lock is exclusive. This is more efficient
// than a plain Mutex when reads (broadcasts to members) far outnumber writes
// (join/leave events).
type room struct {
	mu      sync.RWMutex
	clients map[*client]struct{} // set of clients — map with empty struct values uses minimal memory
}

func newRoom() *room {
	return &room{clients: make(map[*client]struct{})}
}

// add registers a client in the room. Lock/Unlock pairs are the Go equivalent
// of a synchronized block in Java.
func (r *room) add(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock() // defer runs when the enclosing function returns, like finally
	r.clients[c] = struct{}{}
}

func (r *room) remove(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

// broadcast sends a message to every client currently in the room.
// RLock (read lock) allows multiple goroutines to broadcast concurrently
// as long as no one is adding/removing clients at the same time.
func (r *room) broadcast(msg Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		// select is Go's non-blocking channel operation — like a switch, but
		// for channels. The default branch fires immediately if c.send is full,
		// so a single slow client can't block the entire broadcast loop.
		select {
		case c.send <- msg:
		default:
			slog.Warn("dropped message to slow client")
		}
	}
}

// Hub owns all active rooms and the HTTP handler for WebSocket upgrades.
// There is one Hub per server process, created in main.go and shared across
// all requests.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*room // keyed by game code (e.g. "ABC123")
}

// New creates a Hub.
func New() *Hub {
	return &Hub{rooms: make(map[string]*room)}
}

// RegisterRoutes mounts the WebSocket upgrade endpoint on r.
// This endpoint is intentionally unauthenticated — players join with only a
// game code and a display name, no Auth0 account required.
func (h *Hub) RegisterRoutes(r chi.Router) {
	r.Get("/ws/{gameCode}", h.HandleWebSocket)
}

// HandleWebSocket upgrades an HTTP GET to a WebSocket connection and registers
// the client in the room for the given game code. It blocks until the
// connection closes, at which point the client is removed from the room.
//
// The connection lifecycle is:
//  1. HTTP upgrade (websocket.Accept)
//  2. Register client in room
//  3. Spawn a writer goroutine to drain the client's send channel
//  4. Block in the reader loop, dispatching inbound messages
//  5. On disconnect: defer fires, removes client, closes connection
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameCode := chi.URLParam(r, "gameCode")
	if gameCode == "" {
		http.Error(w, "missing game code", http.StatusBadRequest)
		return
	}

	// Accept upgrades the HTTP connection to WebSocket protocol.
	// After this call, w and r should no longer be used directly.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// TODO: restrict OriginPatterns to the production domain before launch.
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}

	c := &client{conn: conn, send: make(chan Message, 64)}
	rm := h.getOrCreateRoom(gameCode)
	rm.add(c)

	// defer is guaranteed to run when HandleWebSocket returns, whether normally
	// or due to an error. This ensures cleanup always happens — equivalent to
	// a finally block.
	defer func() {
		rm.remove(c)
		conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	ctx := r.Context()

	// The writer runs in a separate goroutine (lightweight thread) so it can
	// block waiting for messages without stopping the reader loop.
	// "go func() { ... }()" spawns a goroutine and immediately calls the function.
	// The goroutine exits when c.send is closed or a write error occurs.
	go func() {
		for msg := range c.send {
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				slog.Debug("ws write error", "err", err)
				return
			}
		}
	}()

	// readLoop blocks here, processing inbound messages until the client
	// disconnects. Control only returns to HandleWebSocket when the loop exits,
	// at which point the deferred cleanup above runs.
	h.readLoop(ctx, c, gameCode)
}

// readLoop continuously reads messages from the WebSocket connection and
// dispatches them to handleMessage. Exits when the connection is closed
// (which causes wsjson.Read to return an error).
func (h *Hub) readLoop(ctx context.Context, c *client, gameCode string) {
	for {
		var msg Message
		if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
			slog.Debug("ws read closed", "gameCode", gameCode, "err", err)
			return
		}
		h.handleMessage(ctx, c, gameCode, msg)
	}
}

// handleMessage dispatches an inbound WebSocket message to the appropriate
// handler based on its type. All game business logic will be wired in here
// once the HTTP API layer (game service) is complete.
//
// The leading underscores on _ context.Context and _ *client indicate those
// parameters are not yet used — Go requires all declared variables to be used,
// so _ acknowledges them without causing a compile error.
func (h *Hub) handleMessage(_ context.Context, _ *client, gameCode string, msg Message) {
	slog.Debug("ws message received", "gameCode", gameCode, "type", msg.Type)
	switch msg.Type {
	case MsgJoin:
		// TODO: validate display name, register player in game_players table, broadcast roster update
	case MsgSubmitAnswer:
		// TODO: validate answer is within the time window, persist to answers table, ack to sender
	case MsgStartGame:
		// TODO: verify sender is the host, update game status, broadcast first question
	case MsgAdvanceQuestion:
		// TODO: increment current_question_idx, broadcast next question to room
	case MsgEndGame:
		// TODO: flush final scores from Redis sorted set to Postgres, broadcast game_ended, clean up Redis keys
	default:
		slog.Warn("unknown message type", "type", msg.Type)
	}
}

// Broadcast sends a message to every client in the named room.
// Used by the game service when it needs to push an event triggered by an
// HTTP request (e.g. the host starting the game via POST /games/{id}/start).
func (h *Hub) Broadcast(gameCode string, msg Message) {
	h.mu.RLock()
	rm, ok := h.rooms[gameCode]
	h.mu.RUnlock()
	if !ok {
		return // no active room for this code — game may not have started yet
	}
	rm.broadcast(msg)
}

// getOrCreateRoom returns the existing room for a game code, or creates one
// if it doesn't exist yet. The write lock ensures only one goroutine creates
// the room even if multiple players connect simultaneously.
func (h *Hub) getOrCreateRoom(code string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[code]; ok {
		return r
	}
	r := newRoom()
	h.rooms[code] = r
	return r
}
