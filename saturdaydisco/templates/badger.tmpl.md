/* Badger Email */

{{define "badger_subject"}}Last Call! {{.GameDate}}{{end}}

{{define "badger_body"}}{{- if .Message}}{{.Message}}

{{end}}**We're still short**.  Anyone forget to respond?

{{template "signature" .}}{{end}}

/* Request Badger Approval */

{{define "request_badger_approval_subject"}}[badger-approval-request] Can I badger folks?{{end}}

{{define "request_badger_approval_body"}}Hey boss,

Can I badger folks?

Respond with /approve or /yes or /shipit to send the badger e-mail
Respond with /deny or /no or to do nothing
Respond with /abort to turn off the scheduler and enter manual mode
Ignore this e-mail to have me send the badger eventually

Anything below the your command will be added to the top of the e-mail.

{{template "signature" .}}

Here's our status:

{{template "boss_status" .}}

Here's what I'm thinking:

--- Badger Email ---
Subject: {{template "badger_subject" .}}

{{template "badger_body" .}}{{end}}