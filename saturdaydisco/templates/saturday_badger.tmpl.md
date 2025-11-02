/* Badger Email */

{{define "badger_subject"}}Last Call! {{.GameDate}}{{end}}

{{define "badger_body"}}{{- if .Message}}{{.Message}}

{{end}}**We're still short**.  Here are the folks who've signed up so far: {{.Participants.Public}}

**How do I sign up?**

Just [reply to this e-mail and say "in" if you're coming](mailto:{{.DiscoEmailAddress}}?subject=Re:Saturday Bible Park Frisbee {{.GameDate}}&body=in) .  If you're bringing players with you say something like "[In and bringing 2 others.](mailto:{{.DiscoEmailAddress}}?subject=Re:Saturday Bible Park Frisbee {{.GameDate}}&body=In and bringing 2 others)"

If we missed your reply, please let us know ASAP!

{{template "signature" .}}{{end}}

/* Request Badger Approval */

{{define "request_badger_approval_subject"}}[badger-approval-request] Can I badger folks?{{end}}

{{define "request_badger_approval_body"}}Hey boss,

Can I badger folks?

Respond with /approve or /yes or /shipit to send the badger e-mail
Respond with /deny or /no or to do nothing
Respond with /delay <int> to delay the invite by <int> hours.
Respond with /abort to turn off the scheduler and enter manual mode
Ignore this e-mail to have me send the badger eventually (on {{.NextEvent}})

Anything below the your command will be added to the top of the e-mail.

{{template "signature" .}}

Here's our status:

{{template "boss_status" .}}

Here's what I'm thinking:

--- Badger Email ---
Subject: {{template "badger_subject" .}}

{{template "badger_body" .}}{{end}}