package lunchtimedisco

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/weather"
	"github.com/onsi/say"
)

var GameKeys = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"}
var ValidGameKeys = map[string]bool{}
var DT = map[string]time.Duration{
	"A": -4 * day,
	"B": -4*day + 1*time.Hour,
	"C": -4*day + 2*time.Hour,
	"D": -4*day + 3*time.Hour,

	"E": -3 * day,
	"F": -3*day + 1*time.Hour,
	"G": -3*day + 2*time.Hour,
	"H": -3*day + 3*time.Hour,

	"I": -2 * day,
	"J": -2*day + 1*time.Hour,
	"K": -2*day + 2*time.Hour,
	"L": -2*day + 3*time.Hour,

	"M": -1 * day,
	"N": -1*day + 1*time.Hour,
	"O": -1*day + 2*time.Hour,
	"P": -1*day + 3*time.Hour,
}

func init() {
	for _, key := range GameKeys {
		ValidGameKeys[key] = true
	}
}

func BuildGames(w io.Writer, T time.Time, participants LunchtimeParticipants, forecaster weather.ForecasterInt) Games {
	gameParticipants := map[string]mail.EmailAddresses{}
	for _, participant := range participants {
		for _, key := range participant.GameKeys {
			if _, ok := gameParticipants[key]; !ok {
				gameParticipants[key] = mail.EmailAddresses{}
			}
			gameParticipants[key] = append(gameParticipants[key], participant.Address)
		}
	}

	games := Games{}
	for _, key := range GameKeys {
		players := gameParticipants[key]
		if players == nil {
			players = mail.EmailAddresses{}
		}
		startTime := T.Add(DT[key])
		forecast, err := forecaster.ForecastFor(startTime)
		if err != nil {
			say.Fplni(w, 1, "{{red}}failed to get forecast for %s: %s{{/}}", startTime, err)
			forecast = weather.Forecast{}
		}
		games = append(games, Game{
			Key:       key,
			StartTime: T.Add(DT[key]),
			Forecast:  forecast,
			Players:   players,
		})
	}

	return games
}

type Game struct {
	Key       string
	Players   mail.EmailAddresses
	StartTime time.Time
	Forecast  weather.Forecast
}

func (g Game) IsZero() bool {
	return g.Key == ""
}

func (g Game) String() string {
	return fmt.Sprintf("%s - %d - %s - %s", g.Key, g.Count(), g.FullStartTime(), g.Forecast.String())
}

func (g Game) Count() int {
	return len(g.Players)
}

func (g Game) FullStartTime() string {
	return g.StartTime.Format("Monday 1/2 at 3:04pm")
}

func (g Game) FullStartTimeWithAdjustedTime(adjustedTime string) string {
	return g.StartTime.Format("Monday 1/2") + " at " + adjustedTime
}

func (g Game) GameDate() string {
	return g.StartTime.Format("Monday 1/2")
}
func (g Game) GameDay() string {
	return g.StartTime.Format("Mon")
}

func (g Game) GameTime() string {
	return g.StartTime.Format("3PM")
}

func (g Game) PublicParticipants() string {
	if g.Count() == 0 {
		return "No one's signed up yet"
	}

	out := &strings.Builder{}
	for i, participant := range g.Players {
		out.WriteString(participant.Name())
		if i < len(g.Players)-2 {
			out.WriteString(", ")
		} else if i == len(g.Players)-2 {
			out.WriteString(" and ")
		}
	}
	return out.String()
}

func (g Game) TableCell(pickerURL string) string {
	out := &strings.Builder{}
	color := "#f5f5f5"
	if g.Count() >= 5 {
		color = "#c6f7c6"
	} else if g.Count() >= 3 {
		color = "#f0f7c6"
	} else if g.Count() >= 1 {
		color = "#eee"
	}
	out.WriteString(`<td align="center" valign="top">`)
	fmt.Fprintf(out, `<table border="0" cellpadding="10" cellspacing="0" width="100%%" height="100%%" style="background-color:%s;">`, color)
	out.WriteString(`<tr>`)
	fmt.Fprintf(out, `<td style="font-size:1.2em;" align="center" valign="top"><a style="text-decoration:none;" href="%s">%s</a></td>`, pickerURL, g.GameTime())
	out.WriteString(`</tr>`)
	out.WriteString(`<tr>`)
	fmt.Fprintf(out, `<td style="font-size:1.5em;font-weight:bold;" align="center" valign="top"><a style="text-decoration:none;" href="%s">%d</a></td>`, pickerURL, g.Count())
	out.WriteString(`</tr>`)
	out.WriteString("</table></td>")
	return out.String()
}

type Games []Game

func (g Games) Game(key string) Game {
	for _, game := range g {
		if game.Key == key {
			return game
		}
	}
	return Game{}
}

func (g Games) A() Game { return g.Game("A") }
func (g Games) B() Game { return g.Game("B") }
func (g Games) C() Game { return g.Game("C") }
func (g Games) D() Game { return g.Game("D") }
func (g Games) E() Game { return g.Game("E") }
func (g Games) F() Game { return g.Game("F") }
func (g Games) G() Game { return g.Game("G") }
func (g Games) H() Game { return g.Game("H") }
func (g Games) I() Game { return g.Game("I") }
func (g Games) J() Game { return g.Game("J") }
func (g Games) K() Game { return g.Game("K") }
func (g Games) L() Game { return g.Game("L") }
func (g Games) M() Game { return g.Game("M") }
func (g Games) N() Game { return g.Game("N") }
func (g Games) O() Game { return g.Game("O") }
func (g Games) P() Game { return g.Game("P") }
