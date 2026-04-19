// Package realtime manages WebSocket connections and in-process room state.
//
// Architecture (single-replica phase):
//   - One Hub per process, holding all active rooms keyed by game code.
//   - Each room tracks connected clients + live game state (current question,
//     submitted answers, phase). Scores are persisted to Postgres on reveal.
//   - Scale-out path: replace in-process broadcast with Redis Pub/Sub fan-out.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/benbotsford/trivia/internal/store"
)

// MessageType identifies the kind of realtime event.
type MessageType string

const (
	// Server → all clients in room
	MsgLobbyUpdate      MessageType = "lobby_update"      // player joined lobby
	MsgGameStarted      MessageType = "game_started"      // game transitioned to in_progress
	MsgQuestionRevealed MessageType = "question_revealed" // new question shown
	MsgAnswersRevealed  MessageType = "answers_revealed"  // host revealed correct answers + scores
	MsgRoundLeaderboard MessageType = "round_leaderboard" // end-of-round scoreboard
	MsgGameEnded        MessageType = "game_ended"        // final scoreboard

	// Server → individual client
	MsgAnswerAccepted   MessageType = "answer_accepted"   // ack player's submission
	MsgScoreboardUpdate MessageType = "scoreboard_update" // live answer count (host only)

	// Client → server (host actions)
	MsgStartGame       MessageType = "start_game"       // begin game from lobby
	MsgRevealAnswers   MessageType = "reveal_answers"   // score current question, show answers
	MsgAdvanceQuestion MessageType = "advance_question" // move to next state
	MsgEndGame         MessageType = "end_game"         // force-end game

	// Client → server (player actions)
	MsgSubmitAnswer MessageType = "submit_answer" // player submits their answer
)

// Message is the wire format for all WebSocket messages.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// roomPhase tracks the game state machine.
type roomPhase string

const (
	phaseLobby    roomPhase = "lobby"
	phaseQuestion roomPhase = "question"  // question visible, accepting answers
	phaseAnswers  roomPhase = "answers"   // answers revealed, scoring done
	phaseBoard    roomPhase = "leaderboard"
	phaseEnded    roomPhase = "ended"
)

// client represents one connected WebSocket peer.
type client struct {
	conn     *websocket.Conn
	send     chan Message
	isHost   bool
	playerID uuid.UUID // zero value for host clients
}

// submission holds what a player answered for the current question.
type submission struct {
	answer    string
	isCorrect bool
	points    int32
	name      string // player display name (for broadcast)
}

// room holds all state for one active game session.
type room struct {
	mu sync.Mutex

	clients map[*client]struct{}

	// Set when room is initialised (from HTTP create-game response).
	gameID    uuid.UUID
	bankID    uuid.UUID
	roundSize int32

	// Loaded from DB when MsgStartGame is processed.
	questions []store.Question

	// Dynamic game state.
	phase       roomPhase
	currentIdx  int
	submissions map[uuid.UUID]submission // playerID → what they submitted
}

func newRoom(gameID, bankID uuid.UUID, roundSize int32) *room {
	return &room{
		clients:     make(map[*client]struct{}),
		gameID:      gameID,
		bankID:      bankID,
		roundSize:   roundSize,
		phase:       phaseLobby,
		submissions: make(map[uuid.UUID]submission),
	}
}

func (r *room) addClient(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
}

func (r *room) removeClient(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

// broadcast sends msg to every client in the room (non-blocking per client).
func (r *room) broadcast(msg Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		select {
		case c.send <- msg:
		default:
			slog.Warn("dropped message to slow client")
		}
	}
}

// sendTo delivers a message to a single client.
func sendTo(c *client, msg Message) {
	select {
	case c.send <- msg:
	default:
		slog.Warn("dropped direct message to slow client")
	}
}

// Hub owns all active rooms. There is one Hub per server process.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*room // keyed by game code

	q            *store.Queries
	devAuthToken string // dev bypass — never set in production
}

// New creates the Hub. q is used for DB lookups during game events.
func New(q *store.Queries, devAuthToken string) *Hub {
	return &Hub{
		rooms:        make(map[string]*room),
		q:            q,
		devAuthToken: devAuthToken,
	}
}

// RegisterRoutes mounts the WebSocket upgrade endpoint.
func (h *Hub) RegisterRoutes(r chi.Router) {
	r.Get("/ws/{gameCode}", h.handleWebSocket)
}

// InitRoom creates the in-memory room when a game is created via HTTP.
// Called by the game service after persisting the game to the database.
func (h *Hub) InitRoom(gameID, bankID uuid.UUID, code string, roundSize int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rooms[code] = newRoom(gameID, bankID, roundSize)
}

// handleWebSocket upgrades an HTTP connection to WebSocket.
//
// Role is determined by URL query params:
//   - ?host_token={token}  → host connection (validated against devAuthToken)
//   - ?session={token}     → player connection (validated via DB session token)
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameCode := chi.URLParam(r, "gameCode")

	hostToken := r.URL.Query().Get("host_token")
	sessionToken := r.URL.Query().Get("session")

	var isHost bool
	var playerID uuid.UUID

	switch {
	case hostToken != "":
		if hostToken != h.devAuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		isHost = true

	case sessionToken != "":
		player, err := h.q.GetPlayerBySessionToken(r.Context(), sessionToken)
		if err != nil {
			http.Error(w, "invalid session token", http.StatusUnauthorized)
			return
		}
		playerID = player.ID

	default:
		http.Error(w, "must provide host_token or session query param", http.StatusBadRequest)
		return
	}

	rm := h.getRoom(gameCode)
	if rm == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // TODO: restrict to production domain before launch
	})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}

	c := &client{conn: conn, send: make(chan Message, 64), isHost: isHost, playerID: playerID}
	rm.addClient(c)

	defer func() {
		rm.removeClient(c)
		conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	ctx := r.Context()

	// Writer goroutine drains the send channel to the wire.
	go func() {
		for msg := range c.send {
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				slog.Debug("ws write error", "err", err)
				return
			}
		}
	}()

	// Send current question to a player who joins mid-game.
	if !isHost {
		h.sendCurrentState(rm, c)
	}

	h.readLoop(ctx, c, gameCode)
}

// sendCurrentState replays the current question to a player who connects
// after the game has already started.
func (h *Hub) sendCurrentState(rm *room, c *client) {
	rm.mu.Lock()
	phase := rm.phase
	idx := rm.currentIdx
	questions := rm.questions
	roundSize := rm.roundSize
	rm.mu.Unlock()

	if phase == phaseQuestion && len(questions) > idx {
		sendTo(c, buildQuestionMsg(questions[idx], idx, len(questions), roundSize))
	}
}

// readLoop reads from the WebSocket until the connection closes.
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

// handleMessage dispatches inbound WebSocket messages to handlers.
func (h *Hub) handleMessage(ctx context.Context, c *client, gameCode string, msg Message) {
	slog.Debug("ws message", "gameCode", gameCode, "type", msg.Type, "isHost", c.isHost)

	rm := h.getRoom(gameCode)
	if rm == nil {
		return
	}

	switch msg.Type {
	case MsgStartGame:
		if c.isHost {
			h.onStartGame(ctx, rm)
		}
	case MsgRevealAnswers:
		if c.isHost {
			h.onRevealAnswers(ctx, rm)
		}
	case MsgAdvanceQuestion:
		if c.isHost {
			h.onAdvanceQuestion(ctx, rm)
		}
	case MsgEndGame:
		if c.isHost {
			h.onEndGame(ctx, rm)
		}
	case MsgSubmitAnswer:
		if !c.isHost {
			h.onSubmitAnswer(ctx, rm, c, msg.Payload)
		}
	default:
		slog.Warn("unknown ws message type", "type", msg.Type)
	}
}

// --- Host event handlers ---

// onStartGame transitions the room from lobby → question(0).
// Loads all questions from the DB into room memory for fast access during play.
func (h *Hub) onStartGame(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseLobby {
		rm.mu.Unlock()
		return
	}

	questions, err := h.q.ListQuestionsByBank(ctx, rm.bankID)
	if err != nil || len(questions) == 0 {
		slog.Error("onStartGame: failed to load questions", "err", err)
		rm.mu.Unlock()
		return
	}

	rm.questions = questions
	rm.currentIdx = 0
	rm.phase = phaseQuestion
	rm.submissions = make(map[uuid.UUID]submission)
	total := len(questions)
	roundSize := rm.roundSize
	q := questions[0]
	rm.mu.Unlock()

	if _, err := h.q.StartGame(ctx, rm.gameID); err != nil {
		slog.Error("onStartGame: db update failed", "err", err)
	}

	rm.broadcast(mustMarshal(MsgGameStarted, map[string]any{"total": total, "round_size": roundSize}))
	rm.broadcast(buildQuestionMsg(q, 0, total, roundSize))
}

// onRevealAnswers scores the current question and broadcasts who got it right.
func (h *Hub) onRevealAnswers(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseQuestion {
		rm.mu.Unlock()
		return
	}

	q := rm.questions[rm.currentIdx]
	subs := make(map[uuid.UUID]submission, len(rm.submissions))
	for k, v := range rm.submissions {
		subs[k] = v
	}
	rm.phase = phaseAnswers
	rm.mu.Unlock()

	type entry struct {
		DisplayName string `json:"display_name"`
		Answer      string `json:"answer"`
		Correct     bool   `json:"correct"`
		Points      int32  `json:"points"`
	}
	var entries []entry

	for playerID, sub := range subs {
		if sub.isCorrect {
			if _, err := h.q.AddScoreToPlayer(ctx, store.AddScoreToPlayerParams{
				ID:    playerID,
				Score: sub.points,
			}); err != nil {
				slog.Error("onRevealAnswers: add score failed", "err", err)
			}
		}
		entries = append(entries, entry{
			DisplayName: sub.name,
			Answer:      sub.answer,
			Correct:     sub.isCorrect,
			Points:      sub.points,
		})
	}

	rm.broadcast(mustMarshal(MsgAnswersRevealed, map[string]any{
		"correct_answers": correctAnswersFor(q),
		"entries":         entries,
	}))
}

// onAdvanceQuestion moves to the next state after answers have been revealed.
// State transitions:
//
//	answers → next question  (default)
//	answers → round leaderboard  (if end of round and game not over)
//	answers | leaderboard → game ended  (if all questions done)
func (h *Hub) onAdvanceQuestion(ctx context.Context, rm *room) {
	rm.mu.Lock()
	phase := rm.phase
	if phase != phaseAnswers && phase != phaseBoard {
		rm.mu.Unlock()
		return
	}

	nextIdx := rm.currentIdx + 1
	total := len(rm.questions)
	roundSize := int(rm.roundSize)
	currentIdx := rm.currentIdx
	rm.mu.Unlock()

	gameOver := nextIdx >= total
	endOfRound := (currentIdx+1)%roundSize == 0

	if gameOver {
		h.onEndGame(ctx, rm)
		return
	}

	if phase == phaseAnswers && endOfRound {
		// Show round leaderboard; host must advance again to continue.
		h.broadcastLeaderboard(ctx, rm, false)
		rm.mu.Lock()
		rm.phase = phaseBoard
		rm.mu.Unlock()
		return
	}

	// Advance to next question.
	rm.mu.Lock()
	rm.currentIdx = nextIdx
	rm.phase = phaseQuestion
	rm.submissions = make(map[uuid.UUID]submission)
	q := rm.questions[nextIdx]
	roundSize32 := rm.roundSize
	rm.mu.Unlock()

	rm.broadcast(buildQuestionMsg(q, nextIdx, total, roundSize32))
}

// onEndGame marks the game complete and broadcasts the final leaderboard.
func (h *Hub) onEndGame(ctx context.Context, rm *room) {
	rm.mu.Lock()
	rm.phase = phaseEnded
	rm.mu.Unlock()

	if _, err := h.q.EndGame(ctx, rm.gameID); err != nil {
		slog.Error("onEndGame: db update failed", "err", err)
	}
	h.broadcastLeaderboard(ctx, rm, true)
}

// broadcastLeaderboard fetches scores from the DB and broadcasts them.
func (h *Hub) broadcastLeaderboard(ctx context.Context, rm *room, isFinal bool) {
	type entry struct {
		Rank        int    `json:"rank"`
		DisplayName string `json:"display_name"`
		Score       int32  `json:"score"`
	}

	rows, err := h.q.LeaderboardForGame(ctx, rm.gameID)
	if err != nil {
		slog.Error("broadcastLeaderboard: query failed", "err", err)
		return
	}

	entries := make([]entry, len(rows))
	for i, r := range rows {
		entries[i] = entry{Rank: i + 1, DisplayName: r.DisplayName, Score: r.Score}
	}

	msgType := MsgRoundLeaderboard
	if isFinal {
		msgType = MsgGameEnded
	}
	rm.broadcast(mustMarshal(msgType, map[string]any{"entries": entries}))
}

// --- Player event handlers ---

// onSubmitAnswer records a player's answer and notifies the host.
func (h *Hub) onSubmitAnswer(ctx context.Context, rm *room, c *client, payload json.RawMessage) {
	rm.mu.Lock()
	if rm.phase != phaseQuestion {
		rm.mu.Unlock()
		return
	}
	if _, already := rm.submissions[c.playerID]; already {
		rm.mu.Unlock()
		return
	}
	q := rm.questions[rm.currentIdx]
	rm.mu.Unlock()

	var body struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.Answer == "" {
		return
	}

	player, err := h.q.GetPlayer(ctx, c.playerID)
	if err != nil {
		slog.Error("onSubmitAnswer: player lookup failed", "err", err)
		return
	}

	correct := isCorrectAnswer(q, body.Answer)
	var pts int32
	if correct {
		pts = q.Points
	}

	rm.mu.Lock()
	rm.submissions[c.playerID] = submission{
		answer:    body.Answer,
		isCorrect: correct,
		points:    pts,
		name:      player.DisplayName,
	}
	answerCount := len(rm.submissions)
	rm.mu.Unlock()

	// Confirm receipt to the answering player only.
	sendTo(c, mustMarshal(MsgAnswerAccepted, map[string]any{"correct": correct}))

	// Broadcast answer count so the host's UI updates in real time.
	rm.broadcast(mustMarshal(MsgScoreboardUpdate, map[string]any{"answer_count": answerCount}))
}

// --- Helpers ---

// BroadcastPlayerJoined notifies all room members that a new player has joined.
// Called from the HTTP join endpoint after the player is persisted to the DB.
func (h *Hub) BroadcastPlayerJoined(gameCode, displayName string) {
	rm := h.getRoom(gameCode)
	if rm == nil {
		return
	}
	rm.broadcast(mustMarshal(MsgLobbyUpdate, map[string]any{"player_name": displayName}))
}

// buildQuestionMsg constructs the question_revealed message.
// accepted_answers are intentionally excluded to avoid giving away the answer.
// MC choices are sent without the correct flag.
func buildQuestionMsg(q store.Question, idx, total int, roundSize int32) Message {
	round := idx/int(roundSize) + 1
	posInRound := idx%int(roundSize) + 1

	qPayload := map[string]any{
		"id":     q.ID,
		"type":   string(q.Type),
		"prompt": q.Prompt,
		"points": q.Points,
	}

	if q.Type == store.QuestionTypeMultipleChoice && len(q.Choices) > 0 {
		type fullChoice struct {
			Text    string `json:"text"`
			Correct bool   `json:"correct"`
		}
		var full []fullChoice
		if err := json.Unmarshal(q.Choices, &full); err == nil {
			texts := make([]string, len(full))
			for i, f := range full {
				texts[i] = f.Text
			}
			qPayload["choices"] = texts
		}
	}

	return mustMarshal(MsgQuestionRevealed, map[string]any{
		"index":        idx,
		"total":        total,
		"round":        round,
		"pos_in_round": posInRound,
		"round_size":   roundSize,
		"question":     qPayload,
	})
}

// isCorrectAnswer returns true if the submitted answer matches any accepted answer.
// Matching is case-insensitive and trims surrounding whitespace.
func isCorrectAnswer(q store.Question, answer string) bool {
	answer = strings.TrimSpace(answer)
	switch q.Type {
	case store.QuestionTypeText:
		if len(q.Choices) > 0 {
			var accepted []string
			if err := json.Unmarshal(q.Choices, &accepted); err == nil {
				for _, a := range accepted {
					if strings.EqualFold(answer, strings.TrimSpace(a)) {
						return true
					}
				}
				return false
			}
		}
		return strings.EqualFold(answer, strings.TrimSpace(q.CorrectAnswer))

	case store.QuestionTypeMultipleChoice:
		return strings.EqualFold(answer, strings.TrimSpace(q.CorrectAnswer))
	}
	return false
}

// correctAnswersFor returns the answer(s) to reveal when the host shows results.
func correctAnswersFor(q store.Question) []string {
	switch q.Type {
	case store.QuestionTypeText:
		if len(q.Choices) > 0 {
			var accepted []string
			if err := json.Unmarshal(q.Choices, &accepted); err == nil {
				return accepted
			}
		}
		return []string{q.CorrectAnswer}
	case store.QuestionTypeMultipleChoice:
		return []string{q.CorrectAnswer}
	}
	return nil
}

// mustMarshal builds a Message with the given payload. Panics if marshal fails
// since payloads are always well-formed Go values.
func mustMarshal(t MessageType, payload any) Message {
	b, err := json.Marshal(payload)
	if err != nil {
		panic("realtime: marshal failed: " + err.Error())
	}
	return Message{Type: t, Payload: b}
}

// Broadcast sends a message to every client in the named room.
// Used by the game HTTP service to push events triggered by HTTP requests.
func (h *Hub) Broadcast(gameCode string, msg Message) {
	if rm := h.getRoom(gameCode); rm != nil {
		rm.broadcast(msg)
	}
}

func (h *Hub) getRoom(code string) *room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[code]
}
