package lunchtimedisco

import (
	"context"
	"encoding/json"
	"strings"
	"text/template"
	"time"

	"github.com/onsi/disco/askgpt"
	"github.com/onsi/disco/mail"
)

type promptData struct {
	TuesdayDate   string
	WednesdayDate string
	ThursdayDate  string
	FridayDate    string

	CurrentGameKeys string
}

type routeResponseJSON struct {
	UpdateCount      bool `json:"updateCount"`
	WantsStatus      bool `json:"wantsStatus"`
	WantsUnsubscribe bool `json:"wantsUnsubscribe"`
}

var routePromptTemplate = template.Must(template.New("prompt").Parse(`You are an assistant named Disco.  You are receiving an email message from a potential player responding to an invitation to join an ultimate frisbee game this week.  The players are selecting from a set of options labelled by the letters A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P on Tuesday {{.TuesdayDate}}, Wednesday {{.WednesdayDate}}, Thursday {{.ThursdayDate}}, and Friday {{.FridayDate}} between 10am and 2pm. Your goal is to carefully read the emails and produce a machine-readable JSON response.  The only allowed scenarios and responses are as follows:

1. Players may be discussing which options they can or cannot attend.  Players may use individual letters or ranges of letters like "J",  “A,B,E” or “F-H” or “Wednesday” or “Not Tuesday”  or they may refer to a date or a time.  They may also say something like “I’m out” or “Can’t make it this week.”  Your goal is simply to identify if the player is discussing the options they can and cannot make.  If so,  send a JSON response that has a single field “updateCount” set to true.

2. Participants may want to learn about the status of the current game.  They might say something like “status?” or “Is the game on?”  If you believe they want a status update, send a JSON response that has a single field “wantsStatus” set to true.

3. Participants may want to unsubscribe from the list.  If you believe that is what they want, send a JSON response that has a single field “wantsUnsubscribe” set to true.

4. If you’re unsure what the user is requesting, or if you think the email is just banter or random social conversation, send an empty JSON response (i.e. "{}")`))

var gamesPromptTemplate = template.Must(template.New("prompt").Parse(`Your name is Disco.  This is an e-mail from a player stating what time options they can join an ultimate frisbee game.  The available options:

Tuesday {{.TuesdayDate}}
A:10am-11am
B:11am-12pm
C:12pm-1pm
D:1pm-2pm
Wednesday {{.WednesdayDate}}
E:10-11
F:11-12
G:12-1
H:1-2
Thursday {{.ThursdayDate}}
I:10-11
J:11-12
K:12-1
L:1-2
Friday {{.FridayDate}}
M:10-11
N:11-12
O:12-1
P:1-2

Note that players can refer to the options by letter, day, date, or time.  They may use ranges and discuss which slots they can and cannot make.  They may say things like “out” or “can’t make it this week” to select no options. 

{{if .CurrentGameKeys}}This e-mail is from a player who has already stated they can join the following games:{{.CurrentGameKeys}}.  Make sure to carefully evaluate the e-mail in light of this and update the set of options they can make.{{end}}

Your only valid responses are:
- a comma-separated list of the selected option letters
- the word “none” if they have selected no options
- the word “unsure” if you are unsure
`))

const DEFAULT_TIMEOUT = 40 * time.Second
const USER_MESSAGE_CUTOFF = 1000

type LunchtimeInterpreterInt interface {
	InterpretEmail(email mail.Email, T time.Time, currentGameKeys string) (Command, error)
}

type LunchtimeInterpreter struct {
}

func NewLunchtimeInterpreter() *LunchtimeInterpreter {
	return &LunchtimeInterpreter{}
}

func (interpreter *LunchtimeInterpreter) InterpretEmail(email mail.Email, T time.Time, currentGameKeys string) (Command, error) {
	cmd := Command{
		CommandType:  CommandPlayerUnsure,
		Email:        email,
		EmailAddress: email.From,
	}
	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	data := promptData{
		TuesdayDate:   T.Add(DT["A"]).Format("1/2"),
		WednesdayDate: T.Add(DT["E"]).Format("1/2"),
		ThursdayDate:  T.Add(DT["I"]).Format("1/2"),
		FridayDate:    T.Add(DT["M"]).Format("1/2"),

		CurrentGameKeys: currentGameKeys,
	}
	prompt := &strings.Builder{}
	routePromptTemplate.Execute(prompt, data)

	userMessage := email.Text

	resp, err := askgpt.AskGPT3(ctx, prompt.String(), userMessage)

	if err == askgpt.ErrNoChoices {
		return cmd, nil
	} else if err != nil {
		return Command{}, err
	}

	var response routeResponseJSON
	err = json.Unmarshal([]byte(resp), &response)
	if err != nil {
		return Command{}, err
	}

	if response.UpdateCount {
		prompt := &strings.Builder{}
		gamesPromptTemplate.Execute(prompt, data)
		resp, err := askgpt.AskGPT4(ctx, prompt.String(), userMessage)
		resp = strings.ToUpper(strings.TrimSpace(resp))

		if err == askgpt.ErrNoChoices {
			return cmd, nil
		} else if err != nil {
			return Command{}, err
		} else if resp == "UNSURE" || resp == "" {
			return cmd, nil
		}
		cmd.CommandType = CommandPlayerSetGames
		cmd.GameKeyInput = resp
	} else if response.WantsStatus {
		cmd.CommandType = CommandPlayerStatus
	} else if response.WantsUnsubscribe {
		cmd.CommandType = CommandPlayerUnsubscribe
	}
	return cmd, nil
}
