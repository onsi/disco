package mail

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/onsi/disco/config"
)

func SendEmail(config config.Config, email Email) error {
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
	req, err := http.NewRequest("POST", "https://api.forwardemail.net/v1/emails", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.ForwardEmailKey, "")
	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		issue, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to send e-mail: %d - %s", resp.StatusCode, string(issue))
	}
	return nil
}
