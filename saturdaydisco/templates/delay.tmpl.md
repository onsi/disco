{{define "acknowledge_delay_body"}}I've delayed sending {{.Message}}

{{template "boss_status" .}}

{{template "signature" .}}{{end}}
