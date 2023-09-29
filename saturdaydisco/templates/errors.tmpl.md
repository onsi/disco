/* Invalid Reply State - sent if a reply to an approval request arrives after the state has moved on */
{{define "invalid_reply_state_email_body"}}Hey Boss,

You sent me this e-mail, but my current state is: {{.State}}

...which is incompatible. So you were probably too late.

Status:
{{template "boss_status" .}}

{{template "signature" .}}{{end}}

/* Invalid Admin Email - sent if an incoming command or reply e-mail has an issue*/
{{define "invalid_admin_email_body"}}Hey Boss,

You sent me this e-mail but I ran into an issue:
{{.Error}}

Status:
{{template "boss_status" .}}

{{template "signature" .}}{{end}}