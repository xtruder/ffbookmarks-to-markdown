package llm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/xtruder/ffbookmarks-to-markdown/internal/x"
)

type OpenAIClient struct {
	client *openai.Client
	cache  x.Cache
	model  string
}

func NewOpenAIClient(apiKey, baseURL, model string, httpClient *http.Client, cache x.Cache) (*OpenAIClient, error) {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	)

	return &OpenAIClient{
		client: client,
		cache:  cache,
		model:  model,
	}, nil
}

func (c *OpenAIClient) callLLM(ctx context.Context, prompt string) (string, error) {
	// Try cache first
	key := c.getCacheKey(c.model, prompt)
	if cached, ok := c.cache.Get(key); ok {
		slog.Debug("using cached LLM response")
		return cached, nil
	}

	chatCompletion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a markdown content curator. Your task is to clean and restructure markdown content while preserving its essential information and improving its readability. Be thorough and strict in following the cleaning rules."),
			openai.UserMessage(prompt),
		}),
		Model:       openai.F(c.model),
		Temperature: openai.F(0.1),
	})
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	response := strings.TrimSpace(chatCompletion.Choices[0].Message.Content)
	response = strings.TrimPrefix(response, "```markdown\n")
	response = strings.TrimPrefix(response, "```\n")
	response = strings.TrimSuffix(response, "\n```")

	// Cache the result
	if err := c.cache.Set(key, response); err != nil {
		slog.Warn("failed to cache LLM response", "error", err)
	}

	return response, nil
}

func (c *OpenAIClient) getCacheKey(model, prompt string) string {
	data := fmt.Sprintf("%s\n---\n%s", model, prompt)
	hash := sha256.Sum256([]byte(data))
	return base64.URLEncoding.EncodeToString(hash[:])
}
