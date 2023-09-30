package saturdaydisco

import (
	"context"
	"encoding/json"
	"strings"
	"text/template"
	"time"

	"github.com/onsi/disco/mail"
	openai "github.com/sashabaranov/go-openai"
)

type promptData struct {
	Count int
}

type responseJSON struct {
	UpdateCount      bool `json:"updateCount"`
	Count            int  `json:"count"`
	WantsStatus      bool `json:"wantsStatus"`
	WantsUnsubscribe bool `json:"wantsUnsubscribe"`
}

var promptTemplate = template.Must(template.New("prompt").Parse(`You are an assistant named Disco.  You are receiving an email from a potential player responding to an invitation to join an ultimate frisbee game this Saturday.{{if .Count}}  This player has responded previously and said they are bringing a total of {{.Count}} player(s).{{else}}  This is the first time you are hearing from this player.{{end}}

Your goal is to carefully read their email and produce a machine-readable JSON response.  The only allowed scenarios and responses are as follows:

1. Players may declare that they, and potentially others, will or will not join the game.  If so, please return a JSON response that has a "updateCount" field set to true and a "count" field representing the total number of players joining the game.  Only include participants who can make it, if a player indicates they can’t make it don’t count them.{{if .Count}}  Since this player has responded previously, make sure to update their count correctly and return that.{{end}}

Players who are joining might say things like “I’m in”, or "<name> in" or “+1”, or “Looking forward to it”

Players who aren’t joining might say things like “I can’t”, or “Maybe next week”, or “-1” or “0”.

Sometimes players will also mention others joining.  Such as “I’m in, and so is Bob” or “I can’t make it, but Hannah and my daughter can”.  Make sure to carefully determine who is in and who is out.

If the email is indicating that no player can join set "count" to 0.

2. Participants may want to learn about the status of the current game.  They might say something like “status?” or “Is the game on?”  If you believe they want a status update, send a JSON response that has a single field “wantsStatus” set to true.

3. Participants may want to unsubscribe from the list.  If you believe that is what they want, send a JSON response that has a single field “wantsUnsubscribe” set to true.

4. If you’re unsure what the user is requesting, or if you think the email is just banter or random social conversation, send an empty JSON response (i.e. '{}')`))

const DEFAULT_TIMEOUT = 10 * time.Second
const USER_MESSAGE_CUTOFF = 1000

type InterpreterInt interface {
	InterpretEmail(email mail.Email, count int) (Command, error)
}

type Interpreter struct {
	client *openai.Client
}

func NewInterpreter(apiKey string) *Interpreter {
	return &Interpreter{
		client: openai.NewClient(apiKey),
	}
}

func (interpreter *Interpreter) InterpretEmail(email mail.Email, count int) (Command, error) {
	cmd := Command{
		CommandType:  CommandPlayerUnsure,
		Email:        email,
		EmailAddress: email.From,
	}
	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	prompt := &strings.Builder{}
	promptTemplate.Execute(prompt, promptData{Count: count})

	userMessage := email.Text
	if len(userMessage) > USER_MESSAGE_CUTOFF {
		userMessage = userMessage[:USER_MESSAGE_CUTOFF] + "..."
	}

	resp, err := interpreter.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo,
		MaxTokens:   512,
		Temperature: 0,
		TopP:        1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt.String(),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userMessage,
			},
		},
	})

	if err != nil {
		return Command{}, err
	}

	if len(resp.Choices) == 0 {
		return cmd, nil
	}

	var response responseJSON
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &response)
	if err != nil {
		return Command{}, err
	}

	if response.WantsStatus {
		cmd.CommandType = CommandPlayerStatus
	} else if response.WantsUnsubscribe {
		cmd.CommandType = CommandPlayerUnsubscribe
	} else if response.UpdateCount {
		cmd.CommandType = CommandPlayerSetCount
		cmd.Count = response.Count
	}
	return cmd, nil
}
