package mail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type forwardEmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

func (e forwardEmailAddress) asEmailAddress() EmailAddress {
	if e.Name == "" {
		return EmailAddress(e.Address)
	}
	return EmailAddress(e.Name + " <" + e.Address + ">")
}

type forwardEmailAddressField struct {
	Value []forwardEmailAddress `json:"value"`
}

func (e forwardEmailAddressField) asEmailAddresses() []EmailAddress {
	out := []EmailAddress{}
	for _, v := range e.Value {
		out = append(out, v.asEmailAddress())
	}
	return out
}

type forwardEmailHeader struct {
	Key  string `json:"key"`
	Line string `json:"line"`
}

type forwardEmailModel struct {
	From      forwardEmailAddressField `json:"from"`
	To        forwardEmailAddressField `json:"to"`
	CC        forwardEmailAddressField `json:"cc"`
	Subject   string                   `json:"subject"`
	MessageID string                   `json:"messageId"`
	Text      string                   `json:"text"`
	Headers   []forwardEmailHeader     `json:"headerLines"`
}

var emailRegex = `[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+`
var replyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^>.*`),
	regexp.MustCompile(`(?i)^.*on.*(\n)?wrote:$`),
	regexp.MustCompile(`(?i)-+original\s+message-+\s*$$`),
	regexp.MustCompile(`(?i)From:\s*` + emailRegex),
	regexp.MustCompile(`(?i)<` + emailRegex + ">"),
	regexp.MustCompile(`(?i)` + emailRegex + `\s+wrote:`),
}

func ParseIncomingEmail(data []byte) (Email, error) {
	model := forwardEmailModel{}
	err := json.Unmarshal(data, &model)
	if err != nil {
		return Email{}, err
	}
	out := Email{}
	froms := model.From.asEmailAddresses()
	if len(froms) == 0 {
		return Email{}, fmt.Errorf("no from address found")
	}
	out.From = froms[0]
	out.To = model.To.asEmailAddresses()
	out.CC = model.CC.asEmailAddresses()
	out.Subject = model.Subject
	out.MessageID = model.MessageID

	for _, header := range model.Headers {
		if header.Key == "date" {
			out.Date = strings.TrimPrefix(header.Line, "Date: ")
		}
	}

	fullBody := model.Text
	body := &strings.Builder{}
	lines := strings.Split(fullBody, "\n")
	for idx, line := range lines {
		isDelimiter := false
		for _, regex := range replyRegexes {
			if regex.MatchString(line) {
				isDelimiter = true
				break
			}
		}
		if isDelimiter {
			break
		}
		body.WriteString(line)
		if idx < len(lines)-1 {
			body.WriteString("\n")
		}
	}
	out.Text = body.String()

	return out, nil
}
