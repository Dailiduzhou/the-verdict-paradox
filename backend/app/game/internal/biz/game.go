package biz

import (
	"context"
	"math/rand"
	"slices"
	"sort"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type LLMClient interface {
	GenerateAnswer(ctx context.Context, playerName string, chatHistory []*ChatMessage, question string, systemPrompt string) (string, error)
	GenerateVote(ctx context.Context, playerName string, chatHistory []*ChatMessage, candidates []*PlayerInfo) (int64, error)
}

type GameRepo interface {
	SaveSession(ctx context.Context, session *GameSession) error
	LoadSession(ctx context.Context, roomID string) (*GameSession, error)
	DeleteSession(ctx context.Context, roomID string) error
}

type GameUsecase struct {
	gameRepo  GameRepo
	llmClient LLMClient
	log       *log.Helper
}

func NewGameUsecase(gameRepo GameRepo, llmClient LLMClient, logger log.Logger) *GameUsecase {
	return &GameUsecase{
		gameRepo:  gameRepo,
		llmClient: llmClient,
		log:       log.NewHelper(logger),
	}
}

func (uc *GameUsecase) NewGameSession(roomID string, realPlayers []int64, names map[int64]string) *GameSession {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	aiNameOrder := rng.Perm(len(aiNames))

	players := make([]*PlayerInfo, 0, maxPlayers)

	aiID := int64(-1)
	for i := 0; i < 2; i++ {
		players = append(players, &PlayerInfo{
			UserID: aiID,
			Name:   aiNames[aiNameOrder[i]],
			Role:   RoleAI,
			Alive:  true,
			ConnID: "AI",
		})
		aiID--
	}

	idx := rng.Perm(len(realPlayers))

	for i, pidx := range idx {
		var role PlayerRole
		switch {
		case i == 0:
			role = RoleHumanSpy
		default:
			role = RoleHuman
		}
		uid := realPlayers[pidx]
		players = append(players, &PlayerInfo{
			UserID: uid,
			Name:   names[uid],
			Role:   role,
			Alive:  true,
			ConnID: "",
		})
	}

	return &GameSession{
		RoomID:              roomID,
		Phase:               PhaseWaiting,
		Round:               0,
		Players:             players,
		ChatHistory:         make([]*ChatMessage, 0),
		RoundLogs:           make([]*RoundLog, 0),
		UsedQuestions:       make(map[int]bool),
		CurrentRoundAnswers: nil,
		StartedAt:           time.Now(),
	}
}

func (uc *GameUsecase) PickQuestion(session *GameSession) string {
	total := len(questionBank)
	if len(session.UsedQuestions) >= total {
		session.UsedQuestions = make(map[int]bool)
	}

	available := make([]int, 0, total)
	for i := 0; i < total; i++ {
		if !session.UsedQuestions[i] {
			available = append(available, i)
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	picked := available[rng.Intn(len(available))]
	session.UsedQuestions[picked] = true
	return questionBank[picked]
}

func (uc *GameUsecase) SubmitAnswer(session *GameSession, userID int64, answer string) (allDone bool) {
	currentRound := session.Round
	if currentRound == 0 {
		return false
	}

	for _, p := range session.Players {
		if p.UserID == userID && p.Alive {
			session.CurrentRoundAnswers = append(session.CurrentRoundAnswers, &ChatMessage{
				Round:   currentRound,
				Phase:   "answer",
				UserID:  userID,
				Name:    p.Name,
				Content: answer,
			})
			session.AnswerCount++
			break
		}
	}

	aliveCount := uc.countAliveHumanOrSpy(session)
	return session.AnswerCount >= aliveCount
}

func (uc *GameUsecase) RecordAIAnswer(session *GameSession, userID int64, name string, answer string) {
	session.CurrentRoundAnswers = append(session.CurrentRoundAnswers, &ChatMessage{
		Round:   session.Round,
		Phase:   "answer",
		UserID:  userID,
		Name:    name,
		Content: answer,
	})
}

func (uc *GameUsecase) ShuffleAndCommitAnswers(session *GameSession) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(session.CurrentRoundAnswers), func(i, j int) {
		session.CurrentRoundAnswers[i], session.CurrentRoundAnswers[j] = session.CurrentRoundAnswers[j], session.CurrentRoundAnswers[i]
	})

	session.ChatHistory = append(session.ChatHistory, session.CurrentRoundAnswers...)
	session.CurrentRoundAnswers = nil
}

func (uc *GameUsecase) SubmitVote(session *GameSession, voterID, targetID int64) (allDone bool) {
	for _, p := range session.Players {
		if p.UserID == voterID && p.Alive {
			uc.ensureRoundLog(session)
			session.RoundLogs[session.Round-1].Votes[voterID] = targetID
			session.VoteCount++
			break
		}
	}

	aliveCount := uc.countAliveHumanOrSpy(session)
	return session.VoteCount >= aliveCount
}

func (uc *GameUsecase) RecordAIVote(session *GameSession, voterID, targetID int64) {
	uc.ensureRoundLog(session)
	session.RoundLogs[session.Round-1].Votes[voterID] = targetID
}

func (uc *GameUsecase) TallyVotes(session *GameSession) int64 {
	currentRound := session.Round - 1
	if currentRound < 0 || currentRound >= len(session.RoundLogs) {
		return 0
	}

	log := session.RoundLogs[currentRound]
	voteMap := make(map[int64]int)
	for _, target := range log.Votes {
		voteMap[target]++
	}

	var eliminated int64
	maxVotes := 0
	tie := false

	type kv struct {
		ID    int64
		Count int
	}
	var sorted []kv
	for id, count := range voteMap {
		sorted = append(sorted, kv{id, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	if len(sorted) > 0 {
		maxVotes = sorted[0].Count
		if len(sorted) > 1 && sorted[1].Count == maxVotes {
			tie = true
		}
		if !tie {
			eliminated = sorted[0].ID
			log.Eliminated = eliminated
			for _, p := range session.Players {
				if p.UserID == eliminated {
					p.Alive = false
					break
				}
			}
		}
	}

	return eliminated
}

func (uc *GameUsecase) CheckWinCondition(session *GameSession) (gameOver bool, winner string) {
	var humanAlive, aiAlive int
	var spyAlive bool

	for _, p := range session.Players {
		if !p.Alive {
			continue
		}
		switch p.Role {
		case RoleHuman:
			humanAlive++
		case RoleAI:
			aiAlive++
		case RoleHumanSpy:
			spyAlive = true
		}
	}

	if !spyAlive {
		return true, "SPY"
	}
	if humanAlive == 0 {
		return true, "SPY"
	}
	if aiAlive == 0 {
		return true, "HUMAN"
	}
	return false, ""
}

func (uc *GameUsecase) BeginRound(session *GameSession) {
	session.AnswerCount = 0
	session.VoteCount = 0
	session.PendingAI = true
	session.CurrentRoundAnswers = nil
}

func (uc *GameUsecase) AllVotesDone(session *GameSession) bool {
	aliveCount := uc.countAliveHumanOrSpy(session)
	return session.VoteCount >= aliveCount
}

func (uc *GameUsecase) GenerateAIAnswers(ctx context.Context, session *GameSession) error {
	fullHistory := make([]*ChatMessage, 0, len(session.ChatHistory)+len(session.CurrentRoundAnswers))
	fullHistory = append(fullHistory, session.ChatHistory...)
	fullHistory = append(fullHistory, session.CurrentRoundAnswers...)

	for _, p := range session.Players {
		if p.Role != RoleAI || !p.Alive {
			continue
		}

		answer, err := uc.llmClient.GenerateAnswer(ctx, p.Name, fullHistory, session.Question, "")
		if err != nil {
			uc.log.WithContext(ctx).Errorf("AI answer generation failed for %s: %v", p.Name, err)
			answer = "我觉得这个问题很有趣，但我需要更多时间思考。"
		}

		uc.RecordAIAnswer(session, p.UserID, p.Name, answer)
		uc.log.WithContext(ctx).Infof("AI %s answered round %d", p.Name, session.Round)
	}
	session.PendingAI = false
	return nil
}

func (uc *GameUsecase) GenerateAIVotes(ctx context.Context, session *GameSession) error {
	candidates := make([]*PlayerInfo, 0)
	for _, p := range session.Players {
		if p.Alive {
			candidates = append(candidates, p)
		}
	}

	for _, p := range session.Players {
		if p.Role != RoleAI || !p.Alive {
			continue
		}

		voteTarget, err := uc.llmClient.GenerateVote(ctx, p.Name, session.ChatHistory, candidates)
		if err != nil {
			uc.log.WithContext(ctx).Errorf("AI vote generation failed for %s: %v", p.Name, err)
			voteTarget = uc.randomVoteTarget(p.UserID, candidates)
		}

		valid := false
		for _, c := range candidates {
			if c.UserID == voteTarget && c.UserID != p.UserID {
				valid = true
				break
			}
		}
		if !valid {
			voteTarget = uc.randomVoteTarget(p.UserID, candidates)
		}

		uc.RecordAIVote(session, p.UserID, voteTarget)
	}
	return nil
}

func (uc *GameUsecase) randomVoteTarget(excludeID int64, candidates []*PlayerInfo) int64 {
	valid := make([]*PlayerInfo, 0)
	for _, c := range candidates {
		if c.UserID != excludeID {
			valid = append(valid, c)
		}
	}
	if len(valid) == 0 {
		return 0
	}
	return valid[rand.Intn(len(valid))].UserID
}

func (uc *GameUsecase) ensureRoundLog(session *GameSession) {
	idx := session.Round - 1
	for len(session.RoundLogs) <= idx {
		session.RoundLogs = append(session.RoundLogs, &RoundLog{
			Answers: make(map[int64]string),
			Votes:   make(map[int64]int64),
		})
	}
	if session.RoundLogs[idx].Answers == nil {
		session.RoundLogs[idx].Answers = make(map[int64]string)
	}
	if session.RoundLogs[idx].Votes == nil {
		session.RoundLogs[idx].Votes = make(map[int64]int64)
	}
}

func (uc *GameUsecase) countAliveHumanOrSpy(session *GameSession) int {
	count := 0
	for _, p := range session.Players {
		if (p.Role == RoleHuman || p.Role == RoleHumanSpy) && p.Alive {
			count++
		}
	}
	return count
}

func (uc *GameUsecase) GetAliveAIs(session *GameSession) []*PlayerInfo {
	result := make([]*PlayerInfo, 0)
	for _, p := range session.Players {
		if p.Role == RoleAI && p.Alive {
			result = append(result, p)
		}
	}
	return result
}

func (uc *GameUsecase) GetAliveCandidates(session *GameSession) []*PlayerInfo {
	result := make([]*PlayerInfo, 0)
	for _, p := range session.Players {
		if p.Alive {
			result = append(result, p)
		}
	}
	return result
}

func (uc *GameUsecase) FindPlayer(session *GameSession, userID int64) *PlayerInfo {
	idx := slices.IndexFunc(session.Players, func(p *PlayerInfo) bool {
		return p.UserID == userID
	})
	if idx < 0 {
		return nil
	}
	return session.Players[idx]
}
