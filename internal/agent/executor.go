package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"go.uber.org/zap"
)

// Executor wraps the Anthropic Claude API client and provides
// a simple interface for sending prompts and receiving responses.
type Executor struct {
	client anthropic.Client
	logger *zap.Logger
}

// NewExecutor creates a new Executor with an Anthropic API client.
// The SDK reads the ANTHROPIC_API_KEY environment variable automatically.
func NewExecutor(logger *zap.Logger) *Executor {
	client := anthropic.NewClient()
	return &Executor{
		client: client,
		logger: logger,
	}
}

// ExecutionRequest describes a single Claude API call.
type ExecutionRequest struct {
	Model        string
	SystemPrompt string
	Prompt       string
	MaxTokens    int
}

// ExecutionResult holds the response from a Claude API call.
type ExecutionResult struct {
	Output    string
	TokensIn  int
	TokensOut int
	Error     error
}

// Execute sends a prompt to the Claude API and returns the result.
func (e *Executor) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	model := resolveModel(req.Model)

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 8192
	}

	e.logger.Debug("executing claude API call",
		zap.String("model", model),
		zap.Int64("maxTokens", maxTokens),
		zap.Int("promptLen", len(req.Prompt)),
	)

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	msg, err := e.client.Messages.New(ctx, params)
	if err != nil {
		e.logger.Error("claude API call failed", zap.Error(err))
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	var output strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			output.WriteString(block.Text)
		}
	}

	result := &ExecutionResult{
		Output:    output.String(),
		TokensIn:  int(msg.Usage.InputTokens),
		TokensOut: int(msg.Usage.OutputTokens),
	}

	e.logger.Debug("claude API call completed",
		zap.Int("tokensIn", result.TokensIn),
		zap.Int("tokensOut", result.TokensOut),
	)

	return result, nil
}

// resolveModel maps human-friendly model shortnames to full Anthropic model identifiers.
func resolveModel(model string) string {
	switch model {
	case "claude-sonnet":
		return anthropic.ModelClaude3_7SonnetLatest
	case "claude-haiku":
		return anthropic.ModelClaude3_5HaikuLatest
	case "claude-opus":
		return anthropic.ModelClaude3OpusLatest
	default:
		return model
	}
}
