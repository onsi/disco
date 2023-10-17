package mail

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

const DEFAULT_TIMEOUT = 10 * time.Second

type OutboxInt interface {
	SendEmail(Email) error
}

type Outbox struct {
	forwardEmailKey string
	gmailUser       string
	gmailPassword   string
}

func NewOutbox(forwardEmailKey, gmailUser, gmailPassword string) Outbox {
	return Outbox{
		forwardEmailKey: forwardEmailKey,
		gmailUser:       gmailUser,
		gmailPassword:   gmailPassword,
	}
}

func (o Outbox) SendEmail(email Email) error {
	if strings.Contains(email.From.String(), "@sedenverultimate.net") {
		return o.sendViaForwardEmail(email)
	} else if email.From.Equals(EmailAddress(o.gmailUser)) {
		return o.sendViaGmail(email)
	} else {
		return fmt.Errorf("unknown e-mail address: %s", email.From)
	}
}

func (o Outbox) sendViaForwardEmail(email Email) error {
	form := url.Values{}
	form.Add("from", email.From.String())
	for _, to := range email.To {
		form.Add("to", to.String())
	}
	for _, cc := range email.CC {
		form.Add("cc", cc.String())
	}
	if email.Subject != "" {
		form.Add("subject", email.Subject)
	}
	if email.InReplyTo != "" {
		form.Add("inReplyTo", email.InReplyTo)
	}
	if email.Text != "" {
		form.Add("text", email.Text)
	}
	if email.HTML != "" {
		form.Add("html", email.HTML)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.forwardemail.net/v1/emails", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(o.forwardEmailKey, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		issue, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to send e-mail: %d - %s", resp.StatusCode, string(issue))
	}
	return nil
}

func (o Outbox) sendViaGmail(email Email) error {
	d := gomail.NewDialer("smtp.gmail.com", 587, o.gmailUser, o.gmailPassword)
	m := gomail.NewMessage()
	m.SetHeader("From", email.From.String())
	if (len(email.To)) > 0 {
		m.SetHeader("To", email.To.Strings()...)
	}
	if (len(email.CC)) > 0 {
		m.SetHeader("Cc", email.CC.Strings()...)
	}
	m.SetHeader("Subject", email.Subject)
	m.SetBody("text/plain", email.Text)
	if email.HTML != "" {
		m.AddAlternative("text/html", email.HTML)
	}
	return d.DialAndSend(m)
}
