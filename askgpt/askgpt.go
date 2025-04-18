package askgpt

import (
	"context"
	"errors"
	"time"

	"github.com/onsi/disco/config"
	"github.com/sashabaranov/go-openai"
)

const USER_MESSAGE_CUTOFF = 1000
const ATTEMPT_TIMEOUT = time.Second * 5
const RETRY_DELAY = time.Second * 1

var ErrNoChoices = errors.New("No choices returned from GPT")
var client *openai.Client

func init() {
	apiKey := config.LoadConfig().OpenAIKey
	client = openai.NewClient(apiKey)
}

func askGPT(ctx context.Context, model string, prompt string, userMessage string, wantsJSON bool) (string, error) {
	if len(userMessage) > USER_MESSAGE_CUTOFF {
		userMessage = userMessage[:USER_MESSAGE_CUTOFF] + "..."
	}

	responseFormat := &openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONObject,
	}
	if !wantsJSON {
		responseFormat = nil
	}

	for {
		attemptCtx, cancel := context.WithTimeout(ctx, ATTEMPT_TIMEOUT)
		defer cancel()
		resp, err := client.CreateChatCompletion(attemptCtx, openai.ChatCompletionRequest{
			Model:       model,
			MaxTokens:   512,
			Temperature: 0,
			TopP:        1,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: prompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userMessage,
				},
			},
			ResponseFormat: responseFormat,
		})

		if attemptCtx.Err() != nil && ctx.Err() == nil && err != nil {
			//we timed out, but the parent context is still good, so retry
			time.Sleep(RETRY_DELAY)
			continue
		}

		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", ErrNoChoices
		}

		return resp.Choices[0].Message.Content, nil
	}
}

func AskGPT3(ctx context.Context, prompt string, userMessage string) (string, error) {
	return askGPT(ctx, openai.GPT3Dot5Turbo, prompt, userMessage, false)
}

func AskGPT4ForJSON(ctx context.Context, prompt string, userMessage string) (string, error) {
	return askGPT(ctx, "gpt-4.1", prompt, userMessage, true)
}
