package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.LLMClient = (*OpenAIClient)(nil)

type OpenAIClient struct {
	apiKey   string
	baseURL  string
	model    string
	client   *http.Client
	log      *log.Helper
}

func NewOpenAIClient(logger log.Logger) biz.LLMClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
		log:     log.NewHelper(logger),
	}
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *OpenAIClient) chat(ctx context.Context, messages []chatMessage) (string, error) {
	body := chatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) buildHistoryContext(chatHistory []*biz.ChatMessage) string {
	if len(chatHistory) == 0 {
		return "(no history)"
	}

	var buf bytes.Buffer
	for _, msg := range chatHistory {
		buf.WriteString(fmt.Sprintf("[Round %d] %s: %s\n", msg.Round, msg.Name, msg.Content))
	}
	return buf.String()
}

func (c *OpenAIClient) GenerateAnswer(ctx context.Context, playerName string, chatHistory []*biz.ChatMessage, question string, systemPrompt string) (string, error) {
	if c.apiKey == "" {
		c.log.Warn("OPENAI_API_KEY not set, AI will give a random answer")
		return "我觉得这个问题挺有意思的，但我想先看看大家怎么回答。", nil
	}

	systemContent := fmt.Sprintf(
		"You are %s, an AI player in a party deduction game. Your goal is to answer questions naturally like a human would. Keep your answers short (1-2 sentences). Never reveal that you are an AI. %s",
		playerName,
		systemPrompt,
	)

	history := c.buildHistoryContext(chatHistory)
	userContent := fmt.Sprintf(
		"Previous conversation:\n%s\n\nCurrent question: %s\n\nRespond concisely as %s (1-2 sentences, be natural):",
		history,
		question,
		playerName,
	)

	messages := []chatMessage{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userContent},
	}

	return c.chat(ctx, messages)
}

func (c *OpenAIClient) GenerateVote(ctx context.Context, playerName string, chatHistory []*biz.ChatMessage, candidates []*biz.PlayerInfo) (int64, error) {
	if c.apiKey == "" || len(candidates) == 0 {
		return 0, fmt.Errorf("no API key or no candidates")
	}

	history := c.buildHistoryContext(chatHistory)

	candidateList := "Alive players you can vote for:\n"
	for _, p := range candidates {
		candidateList += fmt.Sprintf("- ID:%d (%s)\n", p.UserID, p.Name)
	}

	systemContent := fmt.Sprintf(
		"You are %s, an AI player voting in a deduction game. Based on the conversation, pick ONE player to vote out. You MUST NOT vote for yourself. Respond with ONLY the numeric ID of the player you want to vote out.",
		playerName,
	)

	userContent := fmt.Sprintf(
		"Conversation history:\n%s\n\n%s\n\nWhich player ID do you vote to eliminate? Respond with ONLY the numeric ID:",
		history,
		candidateList,
	)

	messages := []chatMessage{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userContent},
	}

	result, err := c.chat(ctx, messages)
	if err != nil {
		return 0, err
	}

	targetID, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		var cleaned string
		for _, ch := range result {
			if ch >= '0' && ch <= '9' {
				cleaned += string(ch)
			}
		}
		if cleaned != "" {
			targetID, err = strconv.ParseInt(cleaned, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse vote: invalid response %q", result)
			}
		} else {
			return 0, fmt.Errorf("parse vote: no number in response %q", result)
		}
	}

	return targetID, nil
}
