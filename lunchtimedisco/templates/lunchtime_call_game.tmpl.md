/* Game On */

{{define "game_on_subject"}}GAME ON! {{.GameOnGame.FullStartTime}}{{end}}

{{define "game_on_body"}}{{- if .Message}}{{.Message}}

{{end}}We have quorum!  **GAME ON** for **{{.GameOnGame.FullStartTime}}**.

{{template "public_status" .}}

{{template "signature" .}}{{end}}

/* No Game */

{{define "no_game_subject"}}No Lunchtime Game This Week{{end}}

{{define "no_game_body"}}{{- if .Message}}{{.Message}}

{{end}}**No lunchtime game this week**.  We'll try again next week!

Reminder that we also play on Saturdays. Visit [sedenverultimate.net](https://www.sedenverultimate.net) to sign up for the Saturday mailing list.

{{template "signature" .}}{{end}}