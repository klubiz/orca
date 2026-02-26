package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// Executor wraps the local Claude CLI and provides
// a simple interface for sending prompts and receiving responses.
// It uses the user's local Claude subscription instead of a raw API key.
type Executor struct {
	cliBin string // path to the claude binary
	logger *zap.Logger
}

// NewExecutor creates a new Executor that calls the Claude CLI.
// If cliBin is empty, it defaults to "claude" (resolved via PATH).
func NewExecutor(cliBin string, logger *zap.Logger) *Executor {
	if cliBin == "" {
		cliBin = "claude"
	}
	return &Executor{
		cliBin: cliBin,
		logger: logger,
	}
}

// ExecutionRequest describes a single Claude CLI invocation.
type ExecutionRequest struct {
	Model        string
	SystemPrompt string
	Prompt       string
	MaxTokens    int
}

// ExecutionResult holds the response from a Claude CLI call.
type ExecutionResult struct {
	Output    string
	TokensIn  int
	TokensOut int
	CostUSD   float64
	Error     error
}

// cliResponse maps the JSON output of `claude -p --output-format json`.
type cliResponse struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	DurationMs int     `json:"duration_ms"`
	NumTurns   int     `json:"num_turns"`
	TotalCost  float64 `json:"total_cost_usd"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Execute sends a prompt to the Claude CLI in print mode and returns the result.
func (e *Executor) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	args := []string{
		"-p", req.Prompt,
		"--output-format", "json",
	}

	// Model mapping: map orca shortnames to claude CLI model flags.
	if model := resolveModel(req.Model); model != "" {
		args = append(args, "--model", model)
	}

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	e.logger.Debug("executing claude CLI",
		zap.String("bin", e.cliBin),
		zap.String("model", req.Model),
		zap.Int("promptLen", len(req.Prompt)),
	)

	cmd := exec.CommandContext(ctx, e.cliBin, args...)

	// Unset CLAUDECODE env var to allow nested invocation.
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		e.logger.Error("claude CLI failed",
			zap.Error(err),
			zap.String("stderr", errMsg),
		)
		return nil, fmt.Errorf("claude CLI error: %s", strings.TrimSpace(errMsg))
	}

	// Parse JSON response.
	var resp cliResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		e.logger.Error("failed to parse claude CLI output",
			zap.Error(err),
			zap.String("raw", stdout.String()),
		)
		return nil, fmt.Errorf("parsing claude CLI output: %w", err)
	}

	if resp.IsError && resp.Subtype != "error_max_turns" {
		return nil, fmt.Errorf("claude CLI returned error: %s", resp.Result)
	}

	result := &ExecutionResult{
		Output:    resp.Result,
		TokensIn:  resp.Usage.InputTokens,
		TokensOut: resp.Usage.OutputTokens,
		CostUSD:   resp.TotalCost,
	}

	e.logger.Debug("claude CLI call completed",
		zap.Int("tokensIn", result.TokensIn),
		zap.Int("tokensOut", result.TokensOut),
		zap.Float64("costUSD", result.CostUSD),
		zap.Int("durationMs", resp.DurationMs),
	)

	return result, nil
}

// resolveModel maps orca's human-friendly model shortnames to
// Claude CLI --model flag values.
func resolveModel(model string) string {
	switch model {
	case "claude-sonnet":
		return "sonnet"
	case "claude-haiku":
		return "haiku"
	case "claude-opus":
		return "opus"
	default:
		return model
	}
}

// filterEnv returns a copy of env with the given key removed.
func filterEnv(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}
