/* acknowledge_admin_set_count */

{{define "acknowledge_admin_set_count_body"}}I've set {{.Message}}

{{template "boss_status" .}}

{{template "signature" .}}{{end}}

/* acknowledge_player_set_count */

{{define "acknowledge_player_set_count_body"}}Hey there,

Got it.  I've set your count to {{.Message}}.

{{template "signature" .}}{{end}}