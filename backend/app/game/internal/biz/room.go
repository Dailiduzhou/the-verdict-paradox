package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
)

const (
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	writeWait      = 10 * time.Second
	maxMsgSize     = 4096
	maxPlayers     = 6
	RealPlayers    = 4
	answerTimeout  = 180 * time.Second
	voteTimeout    = 180 * time.Second
	resultDisplay  = 5 * time.Second
	waitingDelay   = 6 * time.Second
	reconnectGrace = 2 * time.Minute
)

type Message struct {
	Action       string          `json:"action"`
	Sender       string          `json:"sender"`
	Content      json.RawMessage `json:"content"`
	UserID       int64           `json:"user_id"`
	TargetUserID int64           `json:"target_user_id,omitempty"`
}

type Client struct {
	ID     string
	UserID int64
	Name   string
	Conn   *websocket.Conn
	Room   *Room
	Send   chan []byte
	log    *log.Helper
}

type Room struct {
	ID         string
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client

	Game        *GameSession
	gameUsecase *GameUsecase
	gameRepo    GameRepo
	matchRepo   MatchRepo
	llmClient   LLMClient

	mu         sync.RWMutex
	aiGenMu    sync.Mutex
	manager    *RoomManager
	idleDone   chan struct{}
	phaseTimer *time.Timer
	log        *log.Helper
}

func NewRoom(id string, manager *RoomManager) *Room {
	return &Room{
		ID:          id,
		Clients:     make(map[*Client]bool),
		Broadcast:   make(chan []byte, 256),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		manager:     manager,
		gameUsecase: manager.gameUsecase,
		gameRepo:    manager.gameRepo,
		matchRepo:   manager.matchRepo,
		llmClient:   manager.llmClient,
		idleDone:    make(chan struct{}),
		log:         manager.log,
	}
}

type RoomManager struct {
	rooms       map[string]*Room
	mu          sync.RWMutex
	gameUsecase *GameUsecase
	gameRepo    GameRepo
	matchRepo   MatchRepo
	llmClient   LLMClient
	log         *log.Helper
}

func NewRoomManager(logger log.Logger, gameUsecase *GameUsecase, gameRepo GameRepo, matchRepo MatchRepo, llmClient LLMClient) *RoomManager {
	return &RoomManager{
		rooms:       make(map[string]*Room),
		gameUsecase: gameUsecase,
		gameRepo:    gameRepo,
		matchRepo:   matchRepo,
		llmClient:   llmClient,
		log:         log.NewHelper(logger),
	}
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (rm *RoomManager) HandleWS(w http.ResponseWriter, r *http.Request, roomID string, userIDStr string, userName string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		rm.log.Errorf("websocket upgrade failed: %v", err)
		return err
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		rm.log.Errorf("parse userID failed: %v", err)
		return err
	}

	rm.mu.Lock()
	room, exists := rm.rooms[roomID]
	if !exists {
		room = NewRoom(roomID, rm)
		rm.rooms[roomID] = room
		go room.Run()
	}
	rm.mu.Unlock()

	client := &Client{
		ID:     userIDStr,
		UserID: userID,
		Name:   userName,
		Conn:   conn,
		Room:   room,
		Send:   make(chan []byte, 256),
		log:    rm.log,
	}
	room.Register <- client

	go client.WritePump()
	go client.ReadPump()

	return nil
}

func (rm *RoomManager) RemoveRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if room, ok := rm.rooms[roomID]; ok {
		close(room.idleDone)
	}
	delete(rm.rooms, roomID)
	rm.log.Infof("room [%s] destroyed", roomID)
}

func (rm *RoomManager) HasRoom(roomID string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	_, exists := rm.rooms[roomID]
	return exists
}

// ---- Client pumps ----

func (c *Client) ReadPump() {
	defer func() {
		c.Room.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMsgSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.log.Errorf("ws read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			c.log.Errorf("parse message failed: %v", err)
			continue
		}
		msg.Sender = c.Name
		msg.UserID = c.UserID

		c.Room.handleMessage(c, &msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ---- Room game methods ----

func (r *Room) handleMessage(client *Client, msg *Message) {
	switch msg.Action {
	case "answer":
		r.handleAnswer(client.UserID, msg)
	case "vote":
		r.handleVote(client.UserID, msg.TargetUserID)
	default:
		data, _ := json.Marshal(msg)
		r.Broadcast <- data
	}
}

func (r *Room) handleAnswer(userID int64, msg *Message) {
	r.mu.Lock()
	if r.Game == nil || r.Game.Phase != PhaseAnswer {
		r.mu.Unlock()
		return
	}

	var answerText string
	if err := json.Unmarshal(msg.Content, &answerText); err != nil {
		answerText = string(msg.Content)
	}

	allDone := r.gameUsecase.SubmitAnswer(r.Game, userID, answerText)
	r.mu.Unlock()

	if allDone {
		r.advanceToVote()
	}
}

func (r *Room) handleVote(userID int64, targetUserID int64) {
	r.mu.Lock()
	if r.Game == nil || r.Game.Phase != PhaseVote {
		r.mu.Unlock()
		return
	}

	allDone := r.gameUsecase.SubmitVote(r.Game, userID, targetUserID)
	r.mu.Unlock()

	if allDone {
		r.generateAIVotesAndTally()
	}
}

func (r *Room) advanceToVote() {
	r.aiGenMu.Lock()

	r.mu.Lock()
	if r.Game == nil || r.Game.Phase != PhaseAnswer {
		r.mu.Unlock()
		r.aiGenMu.Unlock()
		return
	}
	game := r.Game
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.gameUsecase.GenerateAIAnswers(ctx, game); err != nil {
		r.log.Errorf("generate AI answers failed: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.aiGenMu.Unlock()

	if r.Game != game || r.Game.Phase != PhaseAnswer {
		return
	}

	r.gameUsecase.ShuffleAndCommitAnswers(r.Game)

	currentRound := r.Game.Round
	answers := make([]map[string]interface{}, 0)
	for _, msg := range r.Game.ChatHistory {
		if msg.Round == currentRound && msg.Phase == "answer" {
			answers = append(answers, map[string]interface{}{
				"user_id": msg.UserID,
				"name":    msg.Name,
				"content": msg.Content,
			})
		}
	}
	r.broadcastGameAction("all_answers", map[string]interface{}{
		"round":   currentRound,
		"answers": answers,
	})

	r.Game.Phase = PhaseVote
	r.Game.VoteCount = 0

	if r.phaseTimer != nil {
		r.phaseTimer.Stop()
	}

	r.broadcastGameAction("phase_change", map[string]interface{}{
		"phase":     PhaseVote.String(),
		"round":     r.Game.Round,
		"timeout_s": 180,
	})

	r.phaseTimer = time.AfterFunc(voteTimeout, func() {
		r.mu.Lock()
		shouldAdvance := r.Game != nil && r.Game.Phase == PhaseVote
		r.mu.Unlock()
		if shouldAdvance {
			r.generateAIVotesAndTally()
		}
	})
}

func (r *Room) generateAIVotesAndTally() {
	r.aiGenMu.Lock()

	r.mu.Lock()
	if r.Game == nil || r.Game.Phase != PhaseVote {
		r.mu.Unlock()
		r.aiGenMu.Unlock()
		return
	}
	if r.phaseTimer != nil {
		r.phaseTimer.Stop()
	}
	game := r.Game
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.gameUsecase.GenerateAIVotes(ctx, game); err != nil {
		r.log.Errorf("generate AI votes failed: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.aiGenMu.Unlock()

	if r.Game != game || r.Game.Phase != PhaseVote {
		return
	}

	eliminated := r.gameUsecase.TallyVotes(r.Game)
	r.Game.Phase = PhaseResult

	eliminatedName := ""
	for _, p := range r.Game.Players {
		if p.UserID == eliminated {
			eliminatedName = p.Name
			break
		}
	}

	voteData := r.buildVoteData()

	r.broadcastGameAction("round_result", map[string]interface{}{
		"round":         r.Game.Round,
		"eliminated":    eliminatedName,
		"eliminated_id": eliminated,
		"votes":         voteData,
	})

	go r.gameRepo.SaveSession(context.Background(), r.Game)

	gameOver, winner := r.gameUsecase.CheckWinCondition(r.Game)
	if gameOver {
		r.endGame(winner)
		return
	}

	r.phaseTimer = time.AfterFunc(resultDisplay, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.Game == nil || r.Game.Phase != PhaseResult {
			return
		}
		r.Game.Phase = PhaseWaiting
		nextRoundNum := r.Game.Round + 1
		r.broadcastGameAction("phase_change", map[string]interface{}{
			"phase":     PhaseWaiting.String(),
			"round":     nextRoundNum,
			"timeout_s": 6,
		})
		r.phaseTimer = time.AfterFunc(waitingDelay, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.Game != nil && r.Game.Phase == PhaseWaiting {
				r.nextRound()
			}
		})
	})
}

func (r *Room) nextRound() {
	r.Game.Round++
	r.Game.Phase = PhaseQuestion
	r.Game.Question = r.gameUsecase.PickQuestion(r.Game)
	r.gameUsecase.BeginRound(r.Game)

	r.Game.ChatHistory = append(r.Game.ChatHistory, &ChatMessage{
		Round:   r.Game.Round,
		Phase:   "question",
		UserID:  0,
		Name:    "系统",
		Content: r.Game.Question,
	})

	r.broadcastGameAction("question", map[string]interface{}{
		"round":    r.Game.Round,
		"question": r.Game.Question,
	})

	r.Game.Phase = PhaseAnswer
	r.broadcastGameAction("phase_change", map[string]interface{}{
		"phase":     PhaseAnswer.String(),
		"round":     r.Game.Round,
		"timeout_s": 180,
	})

	r.phaseTimer = time.AfterFunc(answerTimeout, func() {
		r.mu.Lock()
		shouldAdvance := r.Game != nil && r.Game.Phase == PhaseAnswer
		r.mu.Unlock()
		if shouldAdvance {
			r.advanceToVote()
		}
	})
}

func (r *Room) endGame(winner string) {
	r.Game.Phase = PhaseEnd
	r.Game.Winner = winner

	if r.phaseTimer != nil {
		r.phaseTimer.Stop()
	}

	playerData := make([]map[string]interface{}, 0, len(r.Game.Players))
	for _, p := range r.Game.Players {
		playerData = append(playerData, map[string]interface{}{
			"user_id": p.UserID,
			"name":    p.Name,
			"role":    p.Role.String(),
			"alive":   p.Alive,
		})
	}

	r.broadcastGameAction("game_over", map[string]interface{}{
		"winner":  winner,
		"players": playerData,
		"rounds":  r.Game.Round,
	})

	go func() {
		time.Sleep(5 * time.Minute)
		r.manager.RemoveRoom(r.ID)
	}()
}

func (r *Room) startGame() {
	r.gameUsecase.BeginRound(r.Game)

	playerList := make([]map[string]interface{}, 0, len(r.Game.Players))
	for _, p := range r.Game.Players {
		playerList = append(playerList, map[string]interface{}{
			"user_id": p.UserID,
			"name":    p.Name,
			"alive":   true,
		})
	}

	for client := range r.Clients {
		var role string
		for _, p := range r.Game.Players {
			if p.UserID == client.UserID {
				role = p.Role.String()
				break
			}
		}

		msg := map[string]interface{}{
			"your_role": role,
			"players":   playerList,
		}
		data, _ := json.Marshal(Message{
			Action: "game_started",
			Sender: "system",
		})
		full := make(map[string]interface{})
		json.Unmarshal(data, &full)
		full["content"] = msg
		final, _ := json.Marshal(full)
		select {
		case client.Send <- final:
		default:
		}
	}

	// Wait 6s before WAITING — role reveal animation plays during this gap.
	r.phaseTimer = time.AfterFunc(waitingDelay, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.Game == nil || r.Game.Phase != PhaseWaiting {
			return
		}

		r.broadcastGameAction("phase_change", map[string]interface{}{
			"phase":     PhaseWaiting.String(),
			"round":     0,
			"timeout_s": 6,
		})

		r.phaseTimer = time.AfterFunc(waitingDelay, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.Game != nil && r.Game.Phase == PhaseWaiting {
				r.nextRound()
			}
		})
	})

	go r.gameRepo.SaveSession(context.Background(), r.Game)
}

func (r *Room) syncGameStateTo(client *Client) {
	if r.Game == nil {
		return
	}

	playerList := make([]map[string]interface{}, 0, len(r.Game.Players))
	for _, p := range r.Game.Players {
		playerList = append(playerList, map[string]interface{}{
			"user_id": p.UserID,
			"name":    p.Name,
			"alive":   p.Alive,
		})
	}

	var role string
	for _, p := range r.Game.Players {
		if p.UserID == client.UserID {
			role = p.Role.String()
			break
		}
	}

	result := map[string]interface{}{
		"action": "game_started",
		"sender": "system",
		"content": map[string]interface{}{
			"your_role": role,
			"players":   playerList,
		},
	}
	data, _ := json.Marshal(result)
	select {
	case client.Send <- data:
	default:
	}

	if r.Game.Phase == PhaseAnswer || r.Game.Phase == PhaseQuestion {
		phaseResult := map[string]interface{}{
			"action": "phase_change",
			"sender": "system",
			"content": map[string]interface{}{
				"phase":     r.Game.Phase.String(),
				"round":     r.Game.Round,
				"timeout_s": 60,
			},
		}
		phaseData, _ := json.Marshal(phaseResult)
		select {
		case client.Send <- phaseData:
		default:
		}
	}

	if r.Game.Question != "" {
		qResult := map[string]interface{}{
			"action": "question",
			"sender": "system",
			"content": map[string]interface{}{
				"round":    r.Game.Round,
				"question": r.Game.Question,
			},
		}
		qData, _ := json.Marshal(qResult)
		select {
		case client.Send <- qData:
		default:
		}
	}
}

func (r *Room) broadcastGameAction(action string, content interface{}) {
	msg := Message{
		Action: action,
		Sender: "system",
	}
	data, _ := json.Marshal(msg)
	full := make(map[string]interface{})
	json.Unmarshal(data, &full)
	full["content"] = content
	final, err := json.Marshal(full)
	if err != nil {
		return
	}

	for client := range r.Clients {
		select {
		case client.Send <- final:
		default:
		}
	}
}

func (r *Room) broadcastSystem(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for client := range r.Clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

func (r *Room) buildVoteData() map[int64]int {
	result := make(map[int64]int)
	if r.Game.Round-1 >= 0 && r.Game.Round-1 < len(r.Game.RoundLogs) {
		roundLog := r.Game.RoundLogs[r.Game.Round-1]
		for _, targetID := range roundLog.Votes {
			result[targetID]++
		}
	}
	return result
}

// ---- Room lifecycle ----

func (r *Room) Run() {
	idleTimer := time.NewTimer(10 * time.Minute)
	defer idleTimer.Stop()

	reconnectTimer := time.NewTimer(reconnectGrace)
	reconnectTimer.Stop()

	for {
		select {
		case client := <-r.Register:
			r.mu.Lock()
			isReconnect := r.handleRegister(client)
			if isReconnect {
				r.mu.Unlock()
			} else {
				r.Clients[client] = true
				count := len(r.Clients)
				r.mu.Unlock()

				// Only broadcast join when game is already running (reconnect).
				r.mu.RLock()
				if r.Game != nil {
					r.mu.RUnlock()
					r.broadcastSystem(Message{
						Action: "player_joined",
						Sender: client.Name,
					})
				} else {
					r.mu.RUnlock()
				}
				r.log.Infof("room [%s] player %s joined, %d total", r.ID, client.Name, count)
			}

			r.mu.RLock()
			allConnected := r.checkAllConnected()
			r.mu.RUnlock()
			if allConnected {
				r.mu.Lock()
				if r.Game == nil {
					r.loadOrCreateGame()
				}
				if r.Game != nil && r.Game.Phase == PhaseWaiting {
					r.startGame()
				}
				r.mu.Unlock()
			}

		case client := <-r.Unregister:
			r.mu.Lock()
			if _, ok := r.Clients[client]; ok {
				delete(r.Clients, client)
				close(client.Send)
			}
			r.handleDisconnect(client)
			count := len(r.Clients)
			r.mu.Unlock()

			r.log.Infof("room [%s] player %s left, %d total", r.ID, client.Name, count)

			if count == 0 {
				r.mu.RLock()
				gameActive := r.Game != nil && r.Game.Phase != PhaseEnd
				r.mu.RUnlock()
				if gameActive {
					reconnectTimer.Reset(reconnectGrace)
				} else {
					reconnectTimer.Reset(10 * time.Minute)
				}
			}

		case <-reconnectTimer.C:
			r.mu.RLock()
			count := len(r.Clients)
			gameActive := r.Game != nil && r.Game.Phase != PhaseEnd
			r.mu.RUnlock()
			if count == 0 {
				if gameActive {
					r.mu.Lock()
					if r.Game != nil && r.Game.Phase != PhaseEnd {
						r.endGame("AI")
						r.mu.Unlock()
					} else {
						r.mu.Unlock()
					}
				} else {
					r.log.Infof("room [%s] idle timeout, destroying", r.ID)
					r.manager.RemoveRoom(r.ID)
					return
				}
			}

		case <-idleTimer.C:
			r.mu.RLock()
			count := len(r.Clients)
			r.mu.RUnlock()
			if count == 0 {
				r.log.Infof("room [%s] idle timeout, destroying", r.ID)
				r.manager.RemoveRoom(r.ID)
				return
			}
			idleTimer.Reset(10 * time.Minute)

		case <-r.idleDone:
			return

		case message := <-r.Broadcast:
			r.mu.RLock()
			for client := range r.Clients {
				select {
				case client.Send <- message:
				default:
					go func(c *Client) {
						c.Room.Unregister <- c
					}(client)
				}
			}
			r.mu.RUnlock()
		}
	}
}

func (r *Room) handleRegister(client *Client) bool {
	if r.Game == nil {
		return false
	}

	for _, p := range r.Game.Players {
		if p.UserID == client.UserID {
			p.ConnID = client.ID
			r.Clients[client] = true
			r.log.Infof("player %s reconnected to room %s", client.Name, r.ID)

			go func() {
				time.Sleep(100 * time.Millisecond)
				r.syncGameStateTo(client)
			}()
			return true
		}
	}
	return false
}

func (r *Room) handleDisconnect(client *Client) {
	if r.Game == nil {
		return
	}

	for _, p := range r.Game.Players {
		if p.UserID == client.UserID {
			p.ConnID = ""
			r.log.Infof("player %s disconnected from game in room %s", p.Name, r.ID)

			// If a real player disconnects, end the game immediately.
			if p.Role != RoleAI && r.Game.Phase != PhaseEnd {
				r.endGame("")

				// Clean up Redis state so the next match is not contaminated.
				go func() {
					ctx := context.Background()
					for _, pl := range r.Game.Players {
						if pl.Role != RoleAI {
							_ = r.matchRepo.ClearPlayerState(ctx, pl.UserID)
						}
					}
					_ = r.matchRepo.DeleteRoomInfo(ctx, r.ID)
					_ = r.gameRepo.DeleteSession(ctx, r.ID)
				}()
				return
			}

			for client2 := range r.Clients {
				if client2.UserID == client.UserID && client2 != client {
					return
				}
			}
			return
		}
	}
}

func (r *Room) checkAllConnected() bool {
	if r.Game == nil {
		return len(r.Clients) >= RealPlayers
	}

	connected := 0
	for _, p := range r.Game.Players {
		if p.Role == RoleAI {
			connected++
			continue
		}
		if p.ConnID != "" {
			connected++
		}
	}

	return connected >= maxPlayers
}

func (r *Room) loadOrCreateGame() {
	ctx := context.Background()
	session, err := r.gameRepo.LoadSession(ctx, r.ID)
	if err != nil {
		r.log.Errorf("load game session failed: %v", err)
	}
	if session != nil {
		session.mu = sync.RWMutex{}
		session.StopCh = make(chan struct{})
		r.Game = session

		for _, p := range r.Game.Players {
			p.ConnID = ""
			for client := range r.Clients {
				if client.UserID == p.UserID {
					p.ConnID = client.ID
					break
				}
			}
			if p.Role == RoleAI {
				p.ConnID = "AI"
			}
		}
		return
	}

	playerIDs := make([]int64, 0, len(r.Clients))
	names := make(map[int64]string)
	for client := range r.Clients {
		playerIDs = append(playerIDs, client.UserID)
		names[client.UserID] = client.Name
	}

	if len(playerIDs) < RealPlayers {
		return
	}

	r.Game = r.gameUsecase.NewGameSession(r.ID, playerIDs, names)
	r.Game.StopCh = make(chan struct{})

	for _, p := range r.Game.Players {
		for client := range r.Clients {
			if client.UserID == p.UserID {
				p.ConnID = client.ID
				break
			}
		}
	}
}
