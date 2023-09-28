/* Invitation */

{{define "invitation_subject"}}Saturday Bible Park Frisbee {{.GameDate}}{{end}}

{{define "invitation_body"}}{{- if .AdditionalContent}}{{.AdditionalContent}}

{{end}}Please let me know if you'll be joining us this Saturday **{{.GameDate}}**.

**Where**: [James Bible Park](https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9)
**When**: Saturday, {{.GameTime}}
**What**: Bring a red and a blue shirt if you have them

Reminder that we also play at lunch during the week. Visit [sedenverultimate.net](https://www.sedenverultimate.net) to sign up for the lunchtime mailing list.

{{template "signature" .}}{{end}}


/* No Invitation */

{{define "no_invitation_subject"}}No Saturday Bible Park Frisbee This Week{{end}}

{{define "no_invitation_body"}}{{- if .AdditionalContent}}{{.AdditionalContent}}

{{end}}No Saturday game this week.  We'll try again next week!

Reminder that we also play at lunch during the week. Visit [sedenverultimate.net](https://www.sedenverultimate.net) to sign up for the lunchtime mailing list.

{{template "signature" .}}{{end}}

/* Request Invite Approval */

{{define "request_invite_approval_subject"}}[invite-approval-request] Can I send this week's invite?{{end}}

{{define "request_invite_approval_body"}}Hey boss,

Can I send this week's invite?

Respond with /approve or /yes or /shipit to send the invite.
Respond with /deny or /no or to send the no-invite e-mail.
Ignore this e-mail to have me send the invite eventually

Anything below the your command will be added to the top of the e-mail.

{{template "signature" .}}

Here's what I'm thinking:

--- Invite Email ---
Subject: {{template "invitation_subject" .}}

{{template "invitation_body" .}}

--- No Invite Email ---
Subject: {{template "no_invitation_subject" .}}

{{template "no_invitation_body" .}}{{end}}