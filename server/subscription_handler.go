package server

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/onsi/disco/mail"
	"github.com/onsi/say"
)

var subscribeTemplate = template.Must(template.New("subscribe").Parse(`Hey boss,

We just got a subscription request:

Email: {{.Email}}
Wants Saturday:  {{.WantsSaturday}}{{if .WantsSaturday}}  Go to: https://groups.google.com/g/saturday-sedenverultimate/members{{end}}
Wants Lunchtime: {{.WantsLunchtime}}{{if .WantsLunchtime}}  Go to: https://groups.google.com/g/southeast-denver-lunchtime-ultimate/members{{end}}

{{if .Message}}Message: {{.Message}}{{end}}

Thanks,

Disco 🪩`))

type SubscriptionRequest struct {
	Email          string `json:"email"`
	WantsSaturday  bool   `json:"wantsSaturday"`
	WantsLunchtime bool   `json:"wantsLunchtime"`
	Message        string `json:"message"`
	TurnstileToken string `json:"turnstileToken"`
}

func truncate(input string, maxLength int) string {
	if len(input) > maxLength {
		input = input[:maxLength] + "..."
	}
	return input
}

func (s *Server) verifyTurnstile(token string) bool {
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify",
		url.Values{"secret": {s.config.TurnstileSecretKey}, "response": {token}})
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	say.Fplni(s.e.Logger.Output(), 0, "{{yellow}}Turnstile response: %s{{/}}", string(data))
	var result struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(data, &result)
	return result.Success
}

func (s *Server) Subscribe(c echo.Context) error {
	say.Fplni(s.e.Logger.Output(), 0, "{{green}}Got a subscription request{{/}}")
	var request SubscriptionRequest
	if err := c.Bind(&request); err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to bind request %s{{/}}", err.Error())
		return c.String(http.StatusBadRequest, err.Error())
	}

	if !s.verifyTurnstile(request.TurnstileToken) {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Turnstile verification failed{{/}}")
		return c.String(http.StatusForbidden, "Human verification failed")
	}

	if !request.WantsSaturday && !request.WantsLunchtime {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}No subscription type selected - ignoring{{/}}")
		return c.String(http.StatusBadRequest, "No subscription type selected")
	}

	request.Email = truncate(strings.TrimSpace(request.Email), 100)
	request.Message = truncate(strings.TrimSpace(request.Message), 1000)

	if request.Email == "" {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Email is required but missing{{/}}")
		return c.String(http.StatusBadRequest, "Email is required")
	}

	body := &strings.Builder{}
	err := subscribeTemplate.Execute(body, request)
	if err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to render email body %s{{/}}", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	err = s.outbox.SendEmail(mail.E().
		WithFrom(s.config.SaturdayDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject("New Subscription Request").WithBody(body.String()))
	if err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to send email %s{{/}}", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	say.Fplni(s.e.Logger.Output(), 1, "{{green}}Sent email{{/}}")
	return c.NoContent(http.StatusOK)
}
