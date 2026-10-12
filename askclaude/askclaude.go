package askclaude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/onsi/disco/config"
)

const MODEL = anthropic.ModelClaudeHaiku5_5
const ATTEMPT_TIMEOUT = time.Second * 5
const RETRY_DELAY = time.Second * 1

var ErrRefusal = errors.New("Claude declined to answer")
var ErrNoAnswer = errors.New("Claude returned no answer")

var client = anthropic.NewClient(option.WithAPIKey(config.LoadConfig().AnthropicKey))

func AskForJSON(ctx context.Context, system string, userMessage string, schema map[string]any, result any) error {
	params := anthropic.MessageNewParams{
		Model:     MODEL,
		MaxTokens: 2048,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	}

	for {
		message, timedOut, err := attempt(ctx, params)
		if timedOut {
			select {
			case <-time.After(RETRY_DELAY):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err != nil {
			return err
		}
		text, err := answer(message)
		if err != nil {
			return err
		}
		return json.Unmarshal([]byte(text), result)
	}
}

func attempt(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, bool, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, ATTEMPT_TIMEOUT)
	defer cancel()
	message, err := client.Messages.New(attemptCtx, params)
	timedOut := err != nil && attemptCtx.Err() != nil && ctx.Err() == nil
	return message, timedOut, err
}

func answer(message *anthropic.Message) (string, error) {
	switch message.StopReason {
	case anthropic.StopReasonRefusal:
		return "", ErrRefusal
	case anthropic.StopReasonMaxTokens:
		return "", fmt.Errorf("Claude ran out of tokens before answering")
	}
	for _, block := range message.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			return text.Text, nil
		}
	}
	return "", ErrNoAnswer
}
