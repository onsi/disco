/* Game On */

{{define "reminder_subject"}}Reminder: GAME ON TODAY! {{.GameDate}}{{end}}

{{define "reminder_body"}}Join us, we're playing today!

{{template "game_details" .}}
{{template "public_status" .}}

{{template "signature" .}}{{end}}