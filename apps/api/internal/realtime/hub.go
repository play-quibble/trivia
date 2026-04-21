// Package realtime manages WebSocket connections and in-process room state.
//
// Architecture (single-replica phase):
//   - One Hub per process, holding all active rooms keyed by game code.
//   - Each room tracks connected clients + live game state.
//   - Scale-out path: replace in-process broadcast with Redis Pub/Sub fan-out.
//
// Round-based game flow:
//
//	phaseLobby
//	  → phaseQuestion   (host releases questions one at a time)
//	  → phaseRoundReview (host reviews + can override incorrect answers)
//	  → phaseBoard      (leaderboard shown to everyone)
//	  → phaseQuestion   (next round starts) OR phaseEnded
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
	// Server → all clients
	MsgLobbyUpdate      MessageType = "lobby_update"       // player joined lobby
	MsgGameStarted      MessageType = "game_started"       // game is now in_progress
	MsgQuestionReleased MessageType = "question_released"  // next question revealed in current round
	MsgRoundEnded       MessageType = "round_ended"        // all questions released; waiting for host review
	MsgRoundScores      MessageType = "round_scores"       // host released scores — sent per-player individually
	MsgRoundLeaderboard MessageType = "round_leaderboard"  // end-of-round scoreboard (after scores released)
	MsgGameEnded        MessageType = "game_ended"         // final scoreboard

	// Server → host only
	MsgRoundReview    MessageType = "round_review"    // host's answer-review screen for the round
	MsgOverrideApplied MessageType = "override_applied" // confirms an override was applied

	// Server → player only
	MsgAnswerAccepted   MessageType = "answer_accepted"   // ack player's submission for a question
	MsgScoreboardUpdate MessageType = "scoreboard_update" // live answer count update

	// Client → server (host actions)
	MsgStartGame       MessageType = "start_game"       // begin game from lobby
	MsgReleaseQuestion MessageType = "release_question" // reveal next question in round
	MsgEndRound        MessageType = "end_round"        // end current round, go to review
	MsgOverrideAnswer  MessageType = "override_answer"  // mark a player's answer as correct
	MsgReleaseScores   MessageType = "release_scores"   // finalize round, apply scoring, show leaderboard
	MsgStartNextRound  MessageType = "start_next_round" // advance from leaderboard to next round
	MsgEndGame         MessageType = "end_game"         // force-end game

	// Client → server (player actions)
	MsgSubmitAnswer MessageType = "submit_answer" // player submits answer for a specific question
)

// Message is the wire format for all WebSocket messages.
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// roomPhase tracks the game state machine.
type roomPhase string

const (
	phaseLobby       roomPhase = "lobby"
	phaseQuestion    roomPhase = "question"     // host releasing questions, players answering
	phaseRoundReview roomPhase = "round_review" // host reviewing answers (host-only phase)
	phaseBoard       roomPhase = "leaderboard"  // post-round leaderboard visible to all
	phaseEnded       roomPhase = "ended"
)

// client represents one connected WebSocket peer.
type client struct {
	conn     *websocket.Conn
	send     chan Message
	isHost   bool
	playerID uuid.UUID // zero value for host clients
}

// roundSubmission holds what a player answered for one question in a round.
type roundSubmission struct {
	answer     string
	isCorrect  bool
	points     int32
	name       string // player display name for the host review screen
	overridden bool   // host manually overrode incorrect → correct
}

// room holds all state for one active game session.
type room struct {
	mu sync.Mutex

	clients map[*client]struct{}

	// Identifiers set when the room is initialised.
	gameID uuid.UUID
	quizID uuid.UUID // zero value for bank-based (legacy) games
	bankID uuid.UUID // used by legacy bank-based games only

	// Quiz-based game state.
	// rounds[i].Questions holds the ordered questions for round i (0-indexed).
	rounds []store.RoundWithQuestions

	// currentRound is 0-indexed into rounds.
	currentRound int

	// releasedCount is how many questions from the current round have been
	// broadcast to players so far. Each press of "Release Question" increments this.
	releasedCount int

	// roundSubs[questionID][playerID] = what the player answered for that question.
	// Reset at the start of each round.
	roundSubs map[uuid.UUID]map[uuid.UUID]roundSubmission

	// phase is the current state-machine node.
	phase roomPhase

	// Legacy bank-based fields (used when quizID == uuid.Nil).
	roundSize   int32
	questions   []store.Question
	currentIdx  int
	submissions map[uuid.UUID]legacySubmission
}

// legacySubmission is the old per-question submission type (bank-based games only).
type legacySubmission struct {
	answer    string
	isCorrect bool
	points    int32
	name      string
}

func newRoom(gameID, quizID, bankID uuid.UUID, roundSize int32) *room {
	return &room{
		clients:     make(map[*client]struct{}),
		gameID:      gameID,
		quizID:      quizID,
		bankID:      bankID,
		roundSize:   roundSize,
		phase:       phaseLobby,
		roundSubs:   make(map[uuid.UUID]map[uuid.UUID]roundSubmission),
		submissions: make(map[uuid.UUID]legacySubmission),
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

// broadcast sends msg to every client in the room.
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

// broadcastToHosts sends msg only to host clients.
func (r *room) broadcastToHosts(msg Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		if c.isHost {
			select {
			case c.send <- msg:
			default:
			}
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
	devAuthToken string
}

// New creates the Hub.
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
func (h *Hub) InitRoom(gameID, quizID, bankID uuid.UUID, code string, roundSize int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rooms[code] = newRoom(gameID, quizID, bankID, roundSize)
}

// handleWebSocket upgrades an HTTP connection to WebSocket.
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
		rm = h.tryRestoreRoom(r.Context(), gameCode)
		if rm == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
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

	go func() {
		for msg := range c.send {
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				slog.Debug("ws write error", "err", err)
				return
			}
		}
	}()

	h.sendCurrentState(rm, c)

	h.readLoop(ctx, c, gameCode)
}

// sendCurrentState replays relevant state to a reconnecting client.
func (h *Hub) sendCurrentState(rm *room, c *client) {
	rm.mu.Lock()
	phase := rm.phase
	rounds := rm.rounds
	currentRound := rm.currentRound
	releasedCount := rm.releasedCount
	rm.mu.Unlock()

	switch phase {
	case phaseQuestion:
		if len(rounds) > 0 && currentRound < len(rounds) {
			rnd := rounds[currentRound]
			totalRounds := len(rounds)
			// Re-send all released questions for current round.
			for i := 0; i < releasedCount && i < len(rnd.Questions); i++ {
				q := rnd.Questions[i]
				sendTo(c, buildQuizQuestionMsg(q, i, len(rnd.Questions), currentRound+1, totalRounds))
			}
		}
	case phaseRoundReview:
		// Only host needs the review screen; players see "waiting" via round_ended.
		if c.isHost && len(rounds) > 0 && currentRound < len(rounds) {
			rm.mu.Lock()
			msg := h.buildRoundReviewMsg(rm)
			rm.mu.Unlock()
			sendTo(c, msg)
		} else if !c.isHost {
			sendTo(c, mustMarshal(MsgRoundEnded, map[string]any{
				"round":       currentRound + 1,
				"total_rounds": len(rounds),
			}))
		}
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

// handleMessage dispatches inbound WebSocket messages.
func (h *Hub) handleMessage(ctx context.Context, c *client, gameCode string, msg Message) {
	slog.Debug("ws message", "gameCode", gameCode, "type", msg.Type, "isHost", c.isHost)

	rm := h.getRoom(gameCode)
	if rm == nil {
		return
	}

	switch msg.Type {
	// Host actions
	case MsgStartGame:
		if c.isHost {
			h.onStartGame(ctx, rm)
		}
	case MsgReleaseQuestion:
		if c.isHost {
			h.onReleaseQuestion(ctx, rm)
		}
	case MsgEndRound:
		if c.isHost {
			h.onEndRound(ctx, rm)
		}
	case MsgOverrideAnswer:
		if c.isHost {
			h.onOverrideAnswer(ctx, rm, msg.Payload)
		}
	case MsgReleaseScores:
		if c.isHost {
			h.onReleaseScores(ctx, rm)
		}
	case MsgStartNextRound:
		if c.isHost {
			h.onStartNextRound(ctx, rm)
		}
	case MsgEndGame:
		if c.isHost {
			h.onEndGame(ctx, rm)
		}

	// Player actions
	case MsgSubmitAnswer:
		if !c.isHost {
			h.onSubmitAnswer(ctx, rm, c, msg.Payload)
		}
	default:
		slog.Warn("unknown ws message type", "type", msg.Type)
	}
}

// ---- Host event handlers (quiz-based) ----------------------------------------

// onStartGame transitions lobby → question, loading all quiz rounds + questions.
func (h *Hub) onStartGame(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseLobby {
		rm.mu.Unlock()
		return
	}

	if rm.quizID == uuid.Nil {
		// Legacy bank-based game — delegate to old handler.
		rm.mu.Unlock()
		h.legacyOnStartGame(ctx, rm)
		return
	}

	rounds, err := h.q.ListQuizRoundsWithQuestions(ctx, rm.quizID)
	if err != nil || len(rounds) == 0 {
		slog.Error("onStartGame: failed to load quiz rounds", "err", err)
		rm.mu.Unlock()
		return
	}

	rm.rounds = rounds
	rm.currentRound = 0
	rm.releasedCount = 0
	rm.phase = phaseQuestion
	rm.roundSubs = make(map[uuid.UUID]map[uuid.UUID]roundSubmission)
	totalRounds := len(rounds)
	rm.mu.Unlock()

	if _, err := h.q.StartGame(ctx, rm.gameID); err != nil {
		slog.Error("onStartGame: db update failed", "err", err)
	}

	rm.broadcast(mustMarshal(MsgGameStarted, map[string]any{
		"total_rounds": totalRounds,
	}))
}

// onReleaseQuestion reveals the next question in the current round.
func (h *Hub) onReleaseQuestion(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseQuestion {
		rm.mu.Unlock()
		return
	}

	if rm.quizID == uuid.Nil {
		rm.mu.Unlock()
		return // not supported in legacy mode
	}

	rounds := rm.rounds
	currentRound := rm.currentRound
	releasedCount := rm.releasedCount

	if currentRound >= len(rounds) {
		rm.mu.Unlock()
		return
	}

	rnd := rounds[currentRound]
	if releasedCount >= len(rnd.Questions) {
		// All questions already released — host should click End Round.
		rm.mu.Unlock()
		return
	}

	rm.releasedCount++
	q := rnd.Questions[releasedCount]
	newReleased := rm.releasedCount
	totalRounds := len(rounds)
	rm.mu.Unlock()

	rm.broadcast(buildQuizQuestionMsg(q, releasedCount, len(rnd.Questions), currentRound+1, totalRounds))

	// Also send the host an answer count update (0 so far for this question).
	rm.broadcastToHosts(mustMarshal(MsgScoreboardUpdate, map[string]any{
		"question_id":  q.ID,
		"answer_count": 0,
		"total_in_round": len(rnd.Questions),
		"released":       newReleased,
	}))
}

// onEndRound moves from phaseQuestion → phaseRoundReview.
// The host sees all player answers; players see "waiting" state.
func (h *Hub) onEndRound(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseQuestion {
		rm.mu.Unlock()
		return
	}
	if rm.quizID == uuid.Nil {
		rm.mu.Unlock()
		return
	}

	rm.phase = phaseRoundReview
	currentRound := rm.currentRound
	totalRounds := len(rm.rounds)
	reviewMsg := h.buildRoundReviewMsg(rm)
	rm.mu.Unlock()

	// Players: inform round is over.
	rm.broadcast(mustMarshal(MsgRoundEnded, map[string]any{
		"round":        currentRound + 1,
		"total_rounds": totalRounds,
	}))

	// Host: send the review screen (overrides message sent above via broadcast too,
	// but host needs the detailed review). We send it separately.
	rm.broadcastToHosts(reviewMsg)
}

// onOverrideAnswer marks a player's answer for a question as correct.
func (h *Hub) onOverrideAnswer(ctx context.Context, rm *room, payload json.RawMessage) {
	var body struct {
		QuestionID string `json:"question_id"`
		PlayerID   string `json:"player_id"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return
	}

	qID, err := uuid.Parse(body.QuestionID)
	if err != nil {
		return
	}
	pID, err := uuid.Parse(body.PlayerID)
	if err != nil {
		return
	}

	rm.mu.Lock()
	if rm.phase != phaseRoundReview {
		rm.mu.Unlock()
		return
	}

	perQ, ok := rm.roundSubs[qID]
	if !ok {
		rm.mu.Unlock()
		return
	}
	sub, ok := perQ[pID]
	if !ok {
		rm.mu.Unlock()
		return
	}

	// Find the question's point value.
	var pts int32
	for _, q := range rm.rounds[rm.currentRound].Questions {
		if q.ID == qID {
			pts = q.Points
			break
		}
	}

	sub.isCorrect = true
	sub.overridden = true
	sub.points = pts
	perQ[pID] = sub
	rm.roundSubs[qID] = perQ

	reviewMsg := h.buildRoundReviewMsg(rm)
	rm.mu.Unlock()

	// Refresh the host's review screen.
	rm.broadcastToHosts(reviewMsg)
}

// onReleaseScores applies scoring for the round, then sends per-player results
// and broadcasts the leaderboard.
func (h *Hub) onReleaseScores(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseRoundReview {
		rm.mu.Unlock()
		return
	}

	if rm.quizID == uuid.Nil {
		rm.mu.Unlock()
		return
	}

	currentRound := rm.currentRound
	totalRounds := len(rm.rounds)
	rnd := rm.rounds[currentRound]

	// Collect all submissions for this round, keyed by playerID.
	type questionResult struct {
		questionID     uuid.UUID
		prompt         string
		correctAnswers []string
		yourAnswer     string
		correct        bool
		pointsEarned   int32
	}

	type playerRoundResult struct {
		playerID        uuid.UUID
		questionResults []questionResult
		totalPoints     int32
	}

	// Build player→client map for direct sends.
	playerClients := make(map[uuid.UUID]*client)
	for c := range rm.clients {
		if !c.isHost {
			playerClients[c.playerID] = c
		}
	}

	// Collect results per player.
	playerResults := make(map[uuid.UUID]*playerRoundResult)

	for _, q := range rnd.Questions {
		perQ := rm.roundSubs[q.ID]
		for playerID, sub := range perQ {
			if _, ok := playerResults[playerID]; !ok {
				playerResults[playerID] = &playerRoundResult{playerID: playerID}
			}
			pr := playerResults[playerID]
			qr := questionResult{
				questionID:    q.ID,
				prompt:        q.Prompt,
				correctAnswers: correctAnswersFor(q),
				yourAnswer:    sub.answer,
				correct:       sub.isCorrect,
				pointsEarned:  sub.points,
			}
			pr.questionResults = append(pr.questionResults, qr)
			if sub.isCorrect {
				pr.totalPoints += sub.points
			}
		}
	}

	rm.phase = phaseBoard
	rm.mu.Unlock()

	// Apply scores to DB.
	for playerID, pr := range playerResults {
		if pr.totalPoints > 0 {
			if _, err := h.q.AddScoreToPlayer(ctx, store.AddScoreToPlayerParams{
				ID:    playerID,
				Score: pr.totalPoints,
			}); err != nil {
				slog.Error("onReleaseScores: add score failed", "err", err)
			}
		}
	}

	// Send per-player round results.
	for playerID, pr := range playerResults {
		c, ok := playerClients[playerID]
		if !ok {
			continue
		}
		type qResultWire struct {
			QuestionID     string   `json:"question_id"`
			Prompt         string   `json:"prompt"`
			CorrectAnswers []string `json:"correct_answers"`
			YourAnswer     string   `json:"your_answer"`
			Correct        bool     `json:"correct"`
			PointsEarned   int32    `json:"points_earned"`
		}
		wireResults := make([]qResultWire, len(pr.questionResults))
		for i, qr := range pr.questionResults {
			wireResults[i] = qResultWire{
				QuestionID:     qr.questionID.String(),
				Prompt:         qr.prompt,
				CorrectAnswers: qr.correctAnswers,
				YourAnswer:     qr.yourAnswer,
				Correct:        qr.correct,
				PointsEarned:   qr.pointsEarned,
			}
		}
		sendTo(c, mustMarshal(MsgRoundScores, map[string]any{
			"round":        currentRound + 1,
			"total_rounds": totalRounds,
			"questions":    wireResults,
			"round_score":  pr.totalPoints,
		}))
	}

	// Also send all correct answers to the host for the scores screen.
	type hostQResult struct {
		QuestionID     string   `json:"question_id"`
		Prompt         string   `json:"prompt"`
		CorrectAnswers []string `json:"correct_answers"`
	}
	hostQResults := make([]hostQResult, len(rnd.Questions))
	for i, q := range rnd.Questions {
		hostQResults[i] = hostQResult{
			QuestionID:     q.ID.String(),
			Prompt:         q.Prompt,
			CorrectAnswers: correctAnswersFor(q),
		}
	}
	rm.broadcastToHosts(mustMarshal(MsgRoundScores, map[string]any{
		"round":        currentRound + 1,
		"total_rounds": totalRounds,
		"questions":    hostQResults,
		"is_host":      true,
	}))

	// Broadcast leaderboard.
	h.broadcastLeaderboard(ctx, rm, false)
}

// onStartNextRound advances from phaseBoard to the next round's phaseQuestion.
func (h *Hub) onStartNextRound(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseBoard {
		rm.mu.Unlock()
		return
	}

	nextRound := rm.currentRound + 1
	if nextRound >= len(rm.rounds) {
		rm.mu.Unlock()
		h.onEndGame(ctx, rm)
		return
	}

	rm.currentRound = nextRound
	rm.releasedCount = 0
	rm.roundSubs = make(map[uuid.UUID]map[uuid.UUID]roundSubmission)
	rm.phase = phaseQuestion
	totalRounds := len(rm.rounds)
	rm.mu.Unlock()

	if _, err := h.q.AdvanceGameRound(ctx, store.AdvanceGameRoundParams{
		ID:              rm.gameID,
		CurrentRoundIdx: int32(nextRound),
	}); err != nil {
		slog.Error("onStartNextRound: db update failed", "err", err)
	}

	rm.broadcast(mustMarshal(MsgGameStarted, map[string]any{
		"round":        nextRound + 1,
		"total_rounds": totalRounds,
	}))
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

	entries := make([]entry, 0, len(rows))
	for i, r := range rows {
		entries = append(entries, entry{Rank: i + 1, DisplayName: r.DisplayName, Score: r.Score})
	}

	rm.mu.Lock()
	currentRound := rm.currentRound
	totalRounds := len(rm.rounds)
	rm.mu.Unlock()

	msgType := MsgRoundLeaderboard
	if isFinal {
		msgType = MsgGameEnded
	}
	rm.broadcast(mustMarshal(msgType, map[string]any{
		"entries":      entries,
		"round":        currentRound + 1,
		"total_rounds": totalRounds,
	}))
}

// ---- Player event handlers ---------------------------------------------------

// onSubmitAnswer records a player's answer for a specific question in the round.
func (h *Hub) onSubmitAnswer(ctx context.Context, rm *room, c *client, payload json.RawMessage) {
	rm.mu.Lock()
	if rm.phase != phaseQuestion {
		rm.mu.Unlock()
		return
	}

	if rm.quizID == uuid.Nil {
		// Legacy path
		rm.mu.Unlock()
		h.legacyOnSubmitAnswer(ctx, rm, c, payload)
		return
	}

	rm.mu.Unlock()

	var body struct {
		QuestionID string `json:"question_id"`
		Answer     string `json:"answer"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.Answer == "" {
		return
	}

	qID, err := uuid.Parse(body.QuestionID)
	if err != nil {
		return
	}

	rm.mu.Lock()
	// Find the question in the current round (must be released).
	rnd := rm.rounds[rm.currentRound]
	var targetQ *store.Question
	for idx, q := range rnd.Questions {
		if q.ID == qID && idx < rm.releasedCount {
			q := q // copy
			targetQ = &q
			break
		}
	}
	if targetQ == nil {
		rm.mu.Unlock()
		return // question not released yet or not in this round
	}

	// Check if player already answered this question.
	if perQ, ok := rm.roundSubs[qID]; ok {
		if _, already := perQ[c.playerID]; already {
			rm.mu.Unlock()
			return
		}
	}
	rm.mu.Unlock()

	player, err := h.q.GetPlayer(ctx, c.playerID)
	if err != nil {
		slog.Error("onSubmitAnswer: player lookup failed", "err", err)
		return
	}

	correct := isCorrectAnswer(*targetQ, body.Answer)
	var pts int32
	if correct {
		pts = targetQ.Points
	}

	rm.mu.Lock()
	if rm.roundSubs[qID] == nil {
		rm.roundSubs[qID] = make(map[uuid.UUID]roundSubmission)
	}
	rm.roundSubs[qID][c.playerID] = roundSubmission{
		answer:    body.Answer,
		isCorrect: correct,
		points:    pts,
		name:      player.DisplayName,
	}

	// Count answers for this question across all players.
	answerCount := len(rm.roundSubs[qID])
	rm.mu.Unlock()

	sendTo(c, mustMarshal(MsgAnswerAccepted, map[string]any{
		"question_id": qID.String(),
		"correct":     correct,
	}))

	rm.broadcastToHosts(mustMarshal(MsgScoreboardUpdate, map[string]any{
		"question_id":  qID.String(),
		"answer_count": answerCount,
	}))
}

// ---- Helpers ---------------------------------------------------------------

// buildRoundReviewMsg constructs the host's review message for the current round.
// Must be called with rm.mu held.
func (h *Hub) buildRoundReviewMsg(rm *room) Message {
	type answerEntry struct {
		PlayerID   string `json:"player_id"`
		PlayerName string `json:"player_name"`
		Answer     string `json:"answer"`
		Correct    bool   `json:"correct"`
		Overridden bool   `json:"overridden"`
	}
	type questionEntry struct {
		QuestionID     string        `json:"question_id"`
		Prompt         string        `json:"prompt"`
		CorrectAnswers []string      `json:"correct_answers"`
		Answers        []answerEntry `json:"answers"`
	}

	rnd := rm.rounds[rm.currentRound]
	questions := make([]questionEntry, 0, len(rnd.Questions))

	for i, q := range rnd.Questions {
		if i >= rm.releasedCount {
			break // only include released questions
		}
		perQ := rm.roundSubs[q.ID]
		answers := make([]answerEntry, 0, len(perQ))
		for playerID, sub := range perQ {
			answers = append(answers, answerEntry{
				PlayerID:   playerID.String(),
				PlayerName: sub.name,
				Answer:     sub.answer,
				Correct:    sub.isCorrect,
				Overridden: sub.overridden,
			})
		}
		questions = append(questions, questionEntry{
			QuestionID:     q.ID.String(),
			Prompt:         q.Prompt,
			CorrectAnswers: correctAnswersFor(q),
			Answers:        answers,
		})
	}

	return mustMarshal(MsgRoundReview, map[string]any{
		"round":        rm.currentRound + 1,
		"total_rounds": len(rm.rounds),
		"questions":    questions,
	})
}

// buildQuizQuestionMsg constructs the question_released message for quiz-based games.
func buildQuizQuestionMsg(q store.Question, posInRound, totalInRound, round, totalRounds int) Message {
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

	return mustMarshal(MsgQuestionReleased, map[string]any{
		"pos_in_round": posInRound + 1, // 1-indexed for display
		"total_in_round": totalInRound,
		"round":          round,
		"total_rounds":   totalRounds,
		"question":       qPayload,
	})
}

// BroadcastPlayerJoined notifies all room members that a new player has joined.
func (h *Hub) BroadcastPlayerJoined(gameCode, displayName string) {
	rm := h.getRoom(gameCode)
	if rm == nil {
		return
	}
	rm.broadcast(mustMarshal(MsgLobbyUpdate, map[string]any{"player_name": displayName}))
}

// Broadcast sends a message to every client in the named room.
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

// tryRestoreRoom re-creates an in-memory room from the database after a restart.
func (h *Hub) tryRestoreRoom(ctx context.Context, code string) *room {
	game, err := h.q.GetActiveGameByCode(ctx, code)
	if err != nil {
		slog.Warn("tryRestoreRoom: game not found in DB", "code", code, "err", err)
		return nil
	}

	var quizID uuid.UUID
	if game.QuizID.Valid {
		quizID = uuid.UUID(game.QuizID.Bytes)
	}

	var bankID uuid.UUID
	if game.BankID.Valid {
		bankID = uuid.UUID(game.BankID.Bytes)
	}

	h.InitRoom(game.ID, quizID, bankID, code, game.RoundSize)
	rm := h.getRoom(code)

	if game.Status == store.GameStatusInProgress {
		if quizID != uuid.Nil {
			// Quiz-based game: reload rounds.
			rounds, err := h.q.ListQuizRoundsWithQuestions(ctx, quizID)
			if err != nil || len(rounds) == 0 {
				slog.Error("tryRestoreRoom: failed to load quiz rounds", "code", code, "err", err)
				return rm
			}
			rm.mu.Lock()
			rm.rounds = rounds
			rm.currentRound = int(game.CurrentRoundIdx)
			// releasedCount unknown after restart — release all questions so host sees them.
			if rm.currentRound < len(rounds) {
				rm.releasedCount = len(rounds[rm.currentRound].Questions)
			}
			rm.phase = phaseQuestion
			rm.roundSubs = make(map[uuid.UUID]map[uuid.UUID]roundSubmission)
			rm.mu.Unlock()
		} else if bankID != uuid.Nil {
			// Legacy bank-based game.
			questions, err := h.q.ListQuestionsByBank(ctx, bankID)
			if err != nil || len(questions) == 0 {
				return rm
			}
			rm.mu.Lock()
			rm.questions = questions
			rm.currentIdx = int(game.CurrentQuestionIdx)
			rm.phase = phaseQuestion
			rm.submissions = make(map[uuid.UUID]legacySubmission)
			rm.mu.Unlock()
		}
		slog.Info("tryRestoreRoom: restored in-progress game", "code", code, "round", game.CurrentRoundIdx)
	}

	return rm
}

// ---- Legacy bank-based handlers (kept for backward compatibility) ------------

func (h *Hub) legacyOnStartGame(ctx context.Context, rm *room) {
	rm.mu.Lock()
	if rm.phase != phaseLobby {
		rm.mu.Unlock()
		return
	}

	questions, err := h.q.ListQuestionsByBank(ctx, rm.bankID)
	if err != nil || len(questions) == 0 {
		slog.Error("legacyOnStartGame: failed to load questions", "err", err)
		rm.mu.Unlock()
		return
	}

	rm.questions = questions
	rm.currentIdx = 0
	rm.phase = phaseQuestion
	rm.submissions = make(map[uuid.UUID]legacySubmission)
	total := len(questions)
	roundSize := rm.roundSize
	q := questions[0]
	rm.mu.Unlock()

	if _, err := h.q.StartGame(ctx, rm.gameID); err != nil {
		slog.Error("legacyOnStartGame: db update failed", "err", err)
	}

	rm.broadcast(mustMarshal(MsgGameStarted, map[string]any{"total": total, "round_size": roundSize}))
	rm.broadcast(legacyBuildQuestionMsg(q, 0, total, roundSize))
}

func (h *Hub) legacyOnSubmitAnswer(ctx context.Context, rm *room, c *client, payload json.RawMessage) {
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
		return
	}

	correct := isCorrectAnswer(q, body.Answer)
	var pts int32
	if correct {
		pts = q.Points
	}

	rm.mu.Lock()
	rm.submissions[c.playerID] = legacySubmission{
		answer:    body.Answer,
		isCorrect: correct,
		points:    pts,
		name:      player.DisplayName,
	}
	answerCount := len(rm.submissions)
	rm.mu.Unlock()

	sendTo(c, mustMarshal(MsgAnswerAccepted, map[string]any{"correct": correct}))
	rm.broadcast(mustMarshal(MsgScoreboardUpdate, map[string]any{"answer_count": answerCount}))
}

// legacyBuildQuestionMsg is the old question_revealed format for bank-based games.
func legacyBuildQuestionMsg(q store.Question, idx, total int, roundSize int32) Message {
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

	return mustMarshal(MsgQuestionReleased, map[string]any{
		"index":        idx,
		"total":        total,
		"round":        round,
		"pos_in_round": posInRound,
		"round_size":   roundSize,
		"question":     qPayload,
	})
}

// ---- Shared helpers ---------------------------------------------------------

// isCorrectAnswer returns true if the submitted answer matches any accepted answer.
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

// mustMarshal builds a Message. Panics if marshal fails.
func mustMarshal(t MessageType, payload any) Message {
	b, err := json.Marshal(payload)
	if err != nil {
		panic("realtime: marshal failed: " + err.Error())
	}
	return Message{Type: t, Payload: b}
}
