/* acknowledge_admin_set_count */

{{define "acknowledge_admin_set_count_body"}}I've set {{.Message}}

{{template "boss_status" .}}

{{template "signature" .}}{{end}}

/* acknowledge_player_set_count */

{{define "acknowledge_player_set_count_body"}}Hey Boss,

I just got the email below.  I've set the player's count to {{.Message}}.  Send me a:

[/set {{.Attachment}} N](mailto:{{.DiscoEmailAddress}}?subject=Set Player&body=/set {{.Attachment}} N)

command if I got it wrong.  Email debug key: {{.EmailDebugKey}}.

{{template "boss_status" .}}

{{template "signature" .}}{{end}}