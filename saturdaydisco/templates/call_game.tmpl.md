/* Game On */

{{define "game_on_subject"}}GAME ON THIS SATURDAY! {{.GameDate}}{{end}}

{{define "game_on_body"}}{{- if .Message}}{{.Message}}

{{end}}We have quorum!  **GAME ON** for **{{.GameDate}}**.

{{template "game_details" .}}
{{template "public_status" .}}

{{template "signature" .}}{{end}}

/* No Game */

{{define "no_game_subject"}}No Saturday Game This Week {{.GameDate}}{{end}}

{{define "no_game_body"}}{{- if .Message}}{{.Message}}

{{end}}No Saturday game this week.  We'll try again next week!

Reminder that we also play at lunch during the week. Visit [sedenverultimate.net](https://www.sedenverultimate.net) to sign up for the lunchtime mailing list.

{{template "signature" .}}{{end}}

/* Request Game On Approval */

{{define "request_game_on_approval_subject"}}[game-on-approval-request] Can I call GAME ON?{{end}}

{{define "request_game_on_approval_body"}}Hey boss,

Can I call game on?

Respond with /approve or /yes or /shipit to send the game on email.
Respond with /deny or /no or **to send the no game e-mail**.
Respond with /delay <int> to delay the invite by <int> hours.
Ignore this e-mail to have me send the game on eventually (on {{.NextEvent}})

Anything below the your command will be added to the top of the e-mail.

{{template "signature" .}}

Here's our status:

{{template "boss_status" .}}

Here's what I'm thinking:

--- Game On Email ---
Subject: {{template "game_on_subject" .}}

{{template "game_on_body" .}}

--- No Game Email ---
Subject: {{template "no_game_subject" .}}

{{template "no_game_body" .}}{{end}}

/* Request No Game Approval */

{{define "request_no_game_approval_subject"}}[no-game-approval-request] Can I call NO GAME?{{end}}

{{define "request_no_game_approval_body"}}Hey boss,

Can I call no game?

Respond with /approve or /yes or /shipit to send the no game email
Respond with /deny or /no **to abort this week**
Respond with /delay <int> to delay the invite by <int> hours.
Respond with /abort to turn off the scheduler and enter manual mode
Ignore this e-mail to have me send the no game eventually (on {{.NextEvent}})

Anything below the your command will be added to the top of the e-mail.

{{template "signature" .}}

Here's our status:

{{template "boss_status" .}}

Here's what I'm thinking:

--- No Game Email ---
Subject: {{template "no_game_subject" .}}

{{template "no_game_body" .}}{{end}}