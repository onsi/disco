/* boss_status the email */
{{define "boss_status_body"}}Hey boss,

Here's the status report.

{{template "boss_status" .}}

{{template "signature" .}}{{end}}

/* boss_status snippet */
{{define "boss_status"}}Current State: {{.State}}

Participants:{{range $idx, $participant := .Participants}}
- {{$participant.Address}}: {{$participant.Count}}
{{$participant.IndentedRelevantEmails}}
{{- end}}

Total Count: {{.Participants.Count}}
Has Quorum: {{.HasQuorum}}

Weather Forecast: {{.Forecast}}{{end}}

Commands: /status, /game-on, /no-game, /abort, /set Player Name <player@example.com> N
Any content on the line below /game-on and /no-game is sent with the e-mail
/abort stops the scheduler but continues to track players and allows you to manually control /game-on and /no-game

/* public_status_body */
{{define "public_status_body"}}Hey there,

It's Disco 🪩.  {{if .GameOn}}**GAME ON!** {{.GameDate}}{{ else if .GameOff}}**NO GAME** {{.GameDate}}{{ else }}The game on {{.GameDate}} hasn't been called yet.{{end}}

{{template "public_status" .}}

{{template "signature" .}}{{end}}

/* public_status */

{{define "public_status"}}**Weather Forecast**: {{.Forecast}}

Players: {{.Participants.Public}}

Total: {{.Participants.Count}}{{end}}

/* game_details */
{{define "game_details"}}**Where**: [James Bible Park](https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9)

**When**: Saturday, {{.GameTime}}

**What**: Bring a red and a blue shirt if you have them
{{end}}
