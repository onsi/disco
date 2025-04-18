package saturdaydisco

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/onsi/disco/askgpt"
	"github.com/onsi/disco/mail"
	"github.com/onsi/say"
)

type promptData struct {
	Count int
}

type responseJSON struct {
	UpdateCount bool `json:"updateCount"`
	Count       int  `json:"count"`
}

var promptTemplate = template.Must(template.New("prompt").Parse(`You are an assistant named Disco.  You are receiving an email from a potential player responding to an invitation to join an ultimate frisbee game this Saturday.{{if .Count}}  This player has responded previously and said they are bringing a total of {{.Count}} player(s).{{else}}  This is the first time you are hearing from this player.{{end}}

Your goal is to carefully read their email and produce a raw machine-readable JSON response.  The response must be valid JSON that can be passed directly to a JSON parser.  The only allowed scenarios and responses are as follows:

1. Players may declare that they, and potentially others, will or will not join the game.  If so, please return a JSON response that has a "updateCount" field set to true and a "count" field representing the total number of players joining the game.  Only include participants who can make it, if a player indicates they can’t make it don’t count them.{{if .Count}}  Since this player has responded previously, make sure to update their count correctly and return that.{{end}}

Players who are joining might say things like "I’m in", or "<name> in" or "+1", or "Looking forward to it"

Players who aren’t joining might say things like "I can’t", or "I'm out" or "Maybe next week", or "-1" or "0".

Sometimes players will also mention others joining.  Such as "I’m in, and so is Bob" or "I can’t make it, but Hannah and my daughter can".  Make sure to carefully determine how many players are in.

If the email is indicating that no player can join set "count" to 0.

2. If you’re unsure what the user is talking about joining or not joining the game (for example, if the email is just banter or random social conversation) send an empty JSON response (i.e. '{}')`))

const DEFAULT_TIMEOUT = 20 * time.Second
const USER_MESSAGE_CUTOFF = 1000

type InterpreterInt interface {
	InterpretEmail(email mail.Email, count int) (Command, error)
}

type Interpreter struct {
	w io.Writer
}

func NewInterpreter(w io.Writer) *Interpreter {
	return &Interpreter{w: w}
}

func (interpreter *Interpreter) InterpretEmail(email mail.Email, count int) (Command, error) {
	cmd := Command{
		CommandType:  CommandPlayerIgnore,
		Email:        email,
		EmailAddress: email.From,
	}
	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	prompt := &strings.Builder{}
	promptTemplate.Execute(prompt, promptData{Count: count})

	userMessage := email.Text
	say.Fplni(interpreter.w, 0, "Asking GPT to interpret email: %s", userMessage)
	resp, err := askgpt.AskGPT4ForJSON(ctx, prompt.String(), userMessage)

	if err == askgpt.ErrNoChoices {
		say.Fplni(interpreter.w, 1, "{{red}}GPT came back with ErrNoChoices{{/}}")
		return cmd, nil
	} else if err != nil {
		say.Fplni(interpreter.w, 1, "{{red}}GPT came back with Error: %s{{/}}", err)
		return Command{}, err
	}

	say.Fplni(interpreter.w, 1, "GPT response: %s", resp)
	var response responseJSON
	err = json.Unmarshal([]byte(resp), &response)
	if err != nil {
		return Command{}, err
	}

	if response.UpdateCount {
		cmd.CommandType = CommandPlayerSetCount
		cmd.Count = response.Count
	}
	return cmd, nil
}
