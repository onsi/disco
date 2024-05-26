/* boss_status the email - a full report with more detail */
{{define "boss_status_body"}}Hey boss,

Here's the status report.

Weather Forecast: {{.Forecast}}
Current State: {{.State}}
Next Event on: {{.NextEvent}}
Total Count: {{.Participants.Count}}
Has Quorum: {{.HasQuorum}}

Participants:{{range $idx, $participant := .Participants}}
- {{$participant.Address}}: {{$participant.Count}}
{{$participant.IndentedRelevantEmails}}
{{- end}}

Commands: /status, /game-on, /no-game, /abort, /set Player Name <player@example.com> N
Any content on the line below /game-on and /no-game is sent with the e-mail
/abort stops the scheduler but continues to track players and allows you to manually control /game-on and /no-game
/RESET-RESET-REST resets the system to pending and drops all the data.  Beware!

{{template "signature" .}}{{end}}

/* boss_status snippet */
{{define "boss_status"}}Current State: {{.State}}
Next Event on: {{.NextEvent}}
Total Count: {{.Participants.Count}}
Has Quorum: {{.HasQuorum}}
Participants:{{range $idx, $participant := .Participants}}
- {{$participant.Address}}: {{$participant.Count}}
{{- end}}{{end}}


/* public_status */

{{define "public_status"}}**Weather Forecast**: {{.Forecast}}

**Players**: {{.Participants.Public}}<br>
**Total**: {{.Participants.Count}}{{if .HasQuorum}} 🎉{{end}}{{end}}

/* game_details */
{{define "game_details"}}**Where**: [James Bible Park](https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9)<br>
**When**: Saturday, {{.GameTime}}<br>
**What**: Bring a red and a blue shirt if you have them<br>{{end}}

{{define "boss_debug_body"}}Hey boss,

This is an e-mail for debugging user-facing templates.

# Public Status

{{template "public_status_body" .}}

---

# Public Invitation

{{template "invitation_body" .}}

---

# Public No Invitation

{{template "no_invitation_body" .}}

---

# Game On

{{template "game_on_body" .}}

---

# No Game

{{template "no_game_body" .}}

---

# Badger

{{template "badger_body" .}}

---{{end}}