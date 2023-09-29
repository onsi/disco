/* acknowledge_admin_set_count */

{{define "acknowledge_admin_set_count_body"}}I've set {{.Message}}

{{template "boss_status" .}}

{{template "signature" .}}{{end}}
