package novacore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type LLMClient struct {
	Provider  string // "anthropic" | "openai" | "openrouter" | custom
	Model     string
	BaseURL   string // override URL base. Si vacío, usa el default del provider.
	APIKeyEnv string // env var donde está la API key. Si vacío, usa el default del provider.
}

func NewLLMClient(provider, model string) *LLMClient {
	return &LLMClient{
		Provider: strings.ToLower(provider),
		Model:    model,
	}
}

func (c *LLMClient) Generate(ctx context.Context, systemPrompt string, history []Message) (string, Usage, error) {
	if c.Provider == "anthropic" {
		return c.generateAnthropic(ctx, systemPrompt, history)
	}
	// openai, openrouter, o cualquier provider con BaseURL configurado
	return c.generateOpenAICompat(ctx, systemPrompt, history)
}

func (c *LLMClient) generateAnthropic(ctx context.Context, systemPrompt string, history []Message) (string, Usage, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", Usage{}, fmt.Errorf("ANTHROPIC_API_KEY no configurado")
	}

	url := "https://api.anthropic.com/v1/messages"
	
	type anthropicMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var anthropicMsgs []anthropicMessage
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	reqBody := map[string]any{
		"model":      c.Model,
		"max_tokens": 1024,
		"messages":   anthropicMsgs,
		"system":     systemPrompt,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", Usage{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("API de Anthropic respondio status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respJSON struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return "", Usage{}, fmt.Errorf("decode response: %w", err)
	}

	if len(respJSON.Content) == 0 || respJSON.Content[0].Type != "text" {
		return "", Usage{}, fmt.Errorf("respuesta sin contenido de texto")
	}

	costIn := float64(respJSON.Usage.InputTokens) * 0.00000025
	costOut := float64(respJSON.Usage.OutputTokens) * 0.00000125
	if strings.Contains(c.Model, "sonnet") {
		costIn = float64(respJSON.Usage.InputTokens) * 0.0000030
		costOut = float64(respJSON.Usage.OutputTokens) * 0.0000150
	}

	return respJSON.Content[0].Text, Usage{
		InputTokens:  respJSON.Usage.InputTokens,
		OutputTokens: respJSON.Usage.OutputTokens,
		CostUSD:      costIn + costOut,
	}, nil
}

func (c *LLMClient) generateOpenAICompat(ctx context.Context, systemPrompt string, history []Message) (string, Usage, error) {
	// Resolver base URL
	baseURL := c.BaseURL
	if baseURL == "" {
		if c.Provider == "openrouter" {
			baseURL = "https://openrouter.ai/api/v1"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}

	// Resolver env var de la API key
	keyEnv := c.APIKeyEnv
	if keyEnv == "" {
		if c.Provider == "openrouter" {
			keyEnv = "OPENROUTER_API_KEY"
		} else {
			keyEnv = "OPENAI_API_KEY"
		}
	}
	apiKey := os.Getenv(keyEnv)
	if apiKey == "" {
		return "", Usage{}, fmt.Errorf("%s no configurado", keyEnv)
	}

	reqURL := baseURL + "/chat/completions"

	type openAIMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var openaiMsgs []openAIMessage
	if systemPrompt != "" {
		openaiMsgs = append(openaiMsgs, openAIMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	for _, m := range history {
		openaiMsgs = append(openaiMsgs, openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := map[string]any{
		"model":    c.Model,
		"messages": openaiMsgs,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", Usage{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("API de %s respondio status %d: %s", c.Provider, resp.StatusCode, string(bodyBytes))
	}

	var respJSON struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respJSON); err != nil {
		return "", Usage{}, fmt.Errorf("decode response: %w", err)
	}

	if len(respJSON.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("respuesta sin elecciones")
	}

	costIn := float64(respJSON.Usage.PromptTokens) * 0.00000015
	costOut := float64(respJSON.Usage.CompletionTokens) * 0.00000060
	if strings.Contains(c.Model, "gpt-4o") && !strings.Contains(c.Model, "mini") {
		costIn = float64(respJSON.Usage.PromptTokens) * 0.0000025
		costOut = float64(respJSON.Usage.CompletionTokens) * 0.0000100
	}

	return respJSON.Choices[0].Message.Content, Usage{
		InputTokens:  respJSON.Usage.PromptTokens,
		OutputTokens: respJSON.Usage.CompletionTokens,
		CostUSD:      costIn + costOut,
	}, nil
}
