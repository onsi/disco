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