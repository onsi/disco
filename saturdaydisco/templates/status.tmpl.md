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
Has Quorum: {{.HasQuorum}}{{end}}

/* public_status */

{{define "public_status"}}Players: {{.Participants.Public}}{{end}}

/* game_details */
{{define "game_details"}}**Where**: [James Bible Park](https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9)
**When**: Saturday, {{.GameTime}}
**What**: Bring a red and a blue shirt if you have them{{end}}
