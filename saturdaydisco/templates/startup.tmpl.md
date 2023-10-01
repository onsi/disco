{{define "startup_error_subject"}}SaturdayDisco FAILED to Join the Dance Floor{{end}}

{{define "startup_error_body"}}Hey Boss,

Something went wrong.  Please take a look!
{{.Error}}

{{template "signature" .}}{{end}}

{{define "startup_subject"}}SaturdayDisco Joined the Dance Floor{{end}}

{{define "startup_body"}}Hey Boss,

I'm up and running now:
{{.Message}}

{{template "boss_status" .}}

{{template "signature" .}}{{end}}

{{define "boss_debug_email"}}Hey boss,

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