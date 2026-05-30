package biz

import (
	"sync"
	"time"
)

type PlayerRole int32

const (
	RoleHuman    PlayerRole = iota // real human
	RoleAI                         // AI bot
	RoleHumanSpy                   // human pretending to be AI
)

func (r PlayerRole) String() string {
	switch r {
	case RoleHuman:
		return "HUMAN"
	case RoleAI:
		return "AI"
	case RoleHumanSpy:
		return "SPY"
	default:
		return "UNKNOWN"
	}
}

type GamePhase int32

const (
	PhaseWaiting  GamePhase = iota // waiting for all players to connect
	PhaseQuestion                  // broadcasting question
	PhaseAnswer                    // collecting answers
	PhaseVote                      // voting
	PhaseResult                    // showing round result
	PhaseEnd                       // game over
)

func (p GamePhase) String() string {
	switch p {
	case PhaseWaiting:
		return "WAITING"
	case PhaseQuestion:
		return "QUESTION"
	case PhaseAnswer:
		return "ANSWER"
	case PhaseVote:
		return "VOTE"
	case PhaseResult:
		return "RESULT"
	case PhaseEnd:
		return "END"
	default:
		return "UNKNOWN"
	}
}

type PlayerInfo struct {
	UserID int64      `json:"user_id"`
	Name   string     `json:"name"`
	Role   PlayerRole `json:"-"`
	Alive  bool       `json:"alive"`
	ConnID string     `json:"-"` // ""=offline, "AI"=AI player, client.ID=online
}

type ChatMessage struct {
	Round   int    `json:"round"`
	Phase   string `json:"phase"`
	UserID  int64  `json:"user_id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type RoundLog struct {
	Round      int              `json:"round"`
	Question   string           `json:"question"`
	Answers    map[int64]string `json:"answers"`
	Votes      map[int64]int64  `json:"votes"`
	Eliminated int64            `json:"eliminated"`
}

type GameSession struct {
	RoomID       string         `json:"room_id"`
	Phase        GamePhase      `json:"phase"`
	Round        int            `json:"round"`
	Question     string         `json:"question"`
	Players      []*PlayerInfo  `json:"players"`
	ChatHistory  []*ChatMessage `json:"chat_history"`
	RoundLogs    []*RoundLog    `json:"round_logs"`
	Winner       string         `json:"winner"`
	StartedAt    time.Time      `json:"started_at"`
	UsedQuestions map[int]bool  `json:"used_questions"`

	AnswerCount int
	VoteCount   int
	PendingAI   bool
	StopCh      chan struct{}

	mu sync.RWMutex
}
