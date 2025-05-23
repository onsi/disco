package mail

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"

	"github.com/google/uuid"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/onsi/say"
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
	ReplyTo   forwardEmailAddressField `json:"replyTo"`
	To        forwardEmailAddressField `json:"to"`
	CC        forwardEmailAddressField `json:"cc"`
	Subject   string                   `json:"subject"`
	MessageID string                   `json:"messageId"`
	Text      string                   `json:"text"`
	HTML      any                      `json:"html"`
	Headers   []forwardEmailHeader     `json:"headerLines"`
}

var emailRegex = `[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+`
var replyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^>.*`),
	regexp.MustCompile(`(?m)^\s*On.*(\s?).*@.*wrote:`),
	regexp.MustCompile(`(?m)^\s*On.*@.*(\s?).*wrote:`),
	regexp.MustCompile(`(?im)-+\s*(original|forwarded)\s+message\s*-+\s*$`),
	regexp.MustCompile(`(?im)From:\s*` + emailRegex),
	regexp.MustCompile(`(?im)` + emailRegex + `\s+wrote:`),
}

func ExtractTopMostPortionFromHTML(htmlBody string) string {
	i := strings.Index(htmlBody, "<blockquote")
	if i >= 0 {
		htmlBody = htmlBody[:i]
	}
	htmlBody = strings.ReplaceAll(htmlBody, "<br", "\n<br")
	txtBody := strip.StripTags(htmlBody)
	return ExtractTopMostPortion(txtBody)

}

func ExtractTopMostPortion(fullBody string) string {
	winner := math.MaxInt
	for _, regex := range replyRegexes {
		index := regex.FindStringIndex(fullBody)
		if index != nil && index[0] < winner {
			winner = index[0]
		}
	}
	if winner == math.MaxInt {
		return fullBody
	}
	return fullBody[:winner]
}

type S3DBInt interface {
	PutObject(key string, data []byte) error
}

func ParseIncomingEmail(db S3DBInt, data []byte, debug io.Writer) (Email, error) {
	//upload e-mail to S3 so we can debug?
	debugKey := "email/" + uuid.New().String()
	say.Fplni(debug, 1, "Email Debugging:  Storing raw email in S3 with key %s", debugKey)
	go func() {
		err := db.PutObject(debugKey, data)
		if err != nil {
			say.Fplni(debug, 2, "{{red}}Email Debugging:  Failed to store key %s{{/}}", debugKey)
		}
	}()

	model := forwardEmailModel{}
	err := json.Unmarshal(data, &model)
	if err != nil {
		return Email{}, err
	}
	out := Email{
		DebugKey: debugKey,
	}
	froms := model.From.asEmailAddresses()
	if len(froms) == 0 {
		return Email{}, fmt.Errorf("no from address found")
	}
	out.From = froms[0]
	if strings.Contains(out.From.Address(), "googlegroups.com") {
		replyTos := model.ReplyTo.asEmailAddresses()
		if len(replyTos) == 0 {
			return Email{}, fmt.Errorf("from address included googlegroups.com, but no reply-to found")
		} else {
			out.From = replyTos[0]
		}
	}
	out.To = model.To.asEmailAddresses()
	out.CC = model.CC.asEmailAddresses()
	out.Subject = model.Subject
	out.MessageID = model.MessageID

	for _, header := range model.Headers {
		if header.Key == "date" {
			out.Date = strings.TrimPrefix(header.Line, "Date: ")
		}
	}

	if model.Text != "" {
		out.Text = ExtractTopMostPortion(model.Text)
	} else {
		switch v := model.HTML.(type) {
		case string:
			if v != "" {
				out.Text = ExtractTopMostPortionFromHTML(v)
			}
		default:
			return Email{}, fmt.Errorf("no content found in email")
		}
	}
	return out, nil
}
