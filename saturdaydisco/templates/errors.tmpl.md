{{define "invalid_reply_state_email_body"}}Hey Boss,

You sent me this e-mail, but my current state is: {{.State}}

...which is incompatible. So you were probably too late.

{{template "signature" .}}{{end}}