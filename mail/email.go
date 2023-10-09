package mail

import (
	"fmt"
	stdlibhtml "html"
	"strings"

	"github.com/client9/gospell/plaintext"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

type Markdown string

type Email struct {
	MessageID string
	InReplyTo string
	DebugKey  string

	From    EmailAddress
	To      EmailAddresses
	CC      EmailAddresses
	Subject string
	Date    string

	Text string
	HTML string
}

func (e Email) Dup() Email {
	return Email{
		MessageID: e.MessageID,
		InReplyTo: e.InReplyTo,
		DebugKey:  e.DebugKey,

		From:    e.From,
		To:      e.To.dup(),
		CC:      e.CC.dup(),
		Subject: e.Subject,
		Date:    e.Date,

		Text: e.Text,
		HTML: e.HTML,
	}
}

func (e Email) String() string {
	return fmt.Sprintf("From: %s on %s\nTo: %s\nCC: %s\nSubject: %s\nDebug Key: %s\n\n%s", e.From, e.Date, e.To, e.CC, e.Subject, e.DebugKey, e.Text)
}

func stripMarkdown(md Markdown) string {
	var mdStripper, _ = plaintext.NewMarkdownText()
	return string(mdStripper.Text([]byte(md)))
}

func renderMarkdown(md Markdown) string {
	var mdParser = parser.NewWithExtensions(parser.CommonExtensions)
	var htmlRenderer = html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})
	return string(markdown.Render(mdParser.Parse([]byte(md)), htmlRenderer))
}

func synthesizeReplyBodies(email Email, body any) (string, string) {
	lines := strings.Split(email.Text, "\n")

	var bodyText string
	var bodyHTML string
	var hasHTML bool

	switch body := body.(type) {
	case Markdown:
		bodyText = stripMarkdown(body)
		bodyHTML = renderMarkdown(body)
		hasHTML = true
	case string:
		bodyText = body
		bodyHTML = ""
		hasHTML = false
	default:
		panic("invalid type for body")
	}

	text := &strings.Builder{}
	text.WriteString(bodyText)
	text.WriteString("\n\n")
	fmt.Fprintf(text, "On %s, %s wrote:\n", email.Date, email.From)
	for idx, line := range lines {
		text.WriteString("> ")
		text.WriteString(line)
		if idx < len(lines)-1 {
			text.WriteString("\n")
		}
	}

	html := &strings.Builder{}
	if hasHTML {
		html.WriteString(bodyHTML)
		fmt.Fprintf(html, "\n<div><blockquote type=\"cite\">On %s, %s wrote:<br><br></blockquote></div>", stdlibhtml.EscapeString(email.Date), stdlibhtml.EscapeString(email.From.String()))
		html.WriteString("\n<blockquote type=\"cite\"><div>")
		for idx, line := range lines {
			html.WriteString(stdlibhtml.EscapeString(line))
			if idx < len(lines)-1 {
				html.WriteString("<br>")
			}
		}
		html.WriteString("</div></blockquote>\n")
	}

	return text.String(), html.String()
}

func E() Email {
	return Email{}
}

func (e Email) WithFrom(from EmailAddress) Email {
	e.From = from
	return e
}

func (e Email) WithTo(to ...EmailAddress) Email {
	e.To = to
	return e
}

func (e Email) AndCC(cc ...EmailAddress) Email {
	e.CC = append(e.CC, cc...)
	return e
}

func (e Email) WithSubject(subject string) Email {
	e.Subject = subject
	return e
}

func (e Email) WithBody(body any) Email {
	switch body := body.(type) {
	case Markdown:
		e.Text = stripMarkdown(body)
		e.HTML = renderMarkdown(body)
	case string:
		e.Text = body
		e.HTML = ""
	default:
		panic("invalid type for body")
	}

	return e
}

func replySubject(subject string) string {
	if strings.HasPrefix(subject, "Re: ") {
		return subject
	}
	return "Re: " + subject
}

func (e Email) Forward(from EmailAddress, to EmailAddress, body any) Email {
	text, html := synthesizeReplyBodies(e, body)
	return Email{
		From:    from,
		To:      EmailAddresses{to},
		Subject: "Fwd: " + e.Subject,
		Text:    text,
		HTML:    html,
	}
}

func (e Email) Reply(from EmailAddress, body any) Email {
	text, html := synthesizeReplyBodies(e, body)
	return Email{
		InReplyTo: e.MessageID,
		From:      from,
		To:        EmailAddresses{e.From},
		Subject:   replySubject(e.Subject),
		Text:      text,
		HTML:      html,
	}
}

func (e Email) ReplyAll(from EmailAddress, body any) Email {
	text, html := synthesizeReplyBodies(e, body)
	ccs := EmailAddresses{}
	for _, to := range e.To {
		if !(to.Equals(from) || to.Equals(e.From)) {
			ccs = append(ccs, to)
		}
	}
	for _, cc := range e.CC {
		if !(cc.Equals(from) || cc.Equals(e.From)) {
			ccs = append(ccs, cc)
		}
	}
	return Email{
		InReplyTo: e.MessageID,
		From:      from,
		To:        EmailAddresses{e.From},
		CC:        ccs,
		Subject:   replySubject(e.Subject),
		Text:      text,
		HTML:      html,
	}
}

func (e Email) ReplyWithoutQuote(from EmailAddress, body any) Email {
	return Email{
		InReplyTo: e.MessageID,
		From:      from,
		To:        EmailAddresses{e.From},
		Subject:   replySubject(e.Subject),
	}.WithBody(body)
}
func (e Email) ReplyAllWithoutQuote(from EmailAddress, body any) Email {
	ccs := EmailAddresses{}
	for _, to := range e.To {
		if !(to.Equals(from) || to.Equals(e.From)) {
			ccs = append(ccs, to)
		}
	}
	for _, cc := range e.CC {
		if !(cc.Equals(from) || cc.Equals(e.From)) {
			ccs = append(ccs, cc)
		}
	}
	return Email{
		InReplyTo: e.MessageID,
		From:      from,
		To:        EmailAddresses{e.From},
		CC:        ccs,
		Subject:   replySubject(e.Subject),
	}.WithBody(body)
}

func (e Email) Recipients() EmailAddresses {
	recipients := EmailAddresses{}
	recipients = append(recipients, e.To...)
	recipients = append(recipients, e.CC...)
	return recipients
}

func (c Email) IncludesRecipient(recipient EmailAddress) bool {
	for _, to := range c.To {
		if to.Equals(recipient) {
			return true
		}
	}
	for _, cc := range c.CC {
		if cc.Equals(recipient) {
			return true
		}
	}
	return false
}
