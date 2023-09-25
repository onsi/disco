package mail

import (
	"strings"

	"github.com/client9/gospell/plaintext"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

type Email struct {
	MessageID string
	InReplyTo string

	From    EmailAddress
	To      []EmailAddress
	CC      []EmailAddress
	Subject string

	Text string
	HTML string
}

func stripMarkdown(md string) string {
	var mdStripper, _ = plaintext.NewMarkdownText()
	return string(mdStripper.Text([]byte(md)))
}

func renderMarkdown(md string) string {
	var mdParser = parser.NewWithExtensions(parser.CommonExtensions)
	var htmlRenderer = html.NewRenderer(html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank,
	})
	return string(markdown.Render(mdParser.Parse([]byte(md)), htmlRenderer))
}

func synthesizeReplyBodies(originalText string, bodyMarkdown string) (string, string) {
	lines := strings.Split(originalText, "\n")

	text := &strings.Builder{}
	text.WriteString(stripMarkdown(bodyMarkdown))
	text.WriteString("\n\n")
	for idx, line := range lines {
		text.WriteString("> ")
		text.WriteString(line)
		if idx < len(lines)-1 {
			text.WriteString("\n")
		}
	}

	html := &strings.Builder{}
	html.WriteString(renderMarkdown(bodyMarkdown))
	html.WriteString("\n<div><blockquote type=\"cite\">")
	for idx, line := range lines {
		html.WriteString(line)
		if idx < len(lines)-1 {
			html.WriteString("<br>")
		}
	}
	html.WriteString("</blockquote></div>\n")

	return text.String(), html.String()
}

func (e Email) WithBody(bodyMarkdown string) Email {
	e.Text = stripMarkdown(bodyMarkdown)
	e.HTML = renderMarkdown(bodyMarkdown)
	return e
}

func (e Email) Reply(from EmailAddress, bodyMarkdown string) Email {
	text, html := synthesizeReplyBodies(e.Text, bodyMarkdown)
	return Email{
		InReplyTo: e.MessageID,
		From:      from,
		To:        []EmailAddress{e.From},
		Subject:   "Re: " + e.Subject,
		Text:      text,
		HTML:      html,
	}
}

func (e Email) ReplyAll(from EmailAddress, bodyMarkdown string) Email {
	text, html := synthesizeReplyBodies(e.Text, bodyMarkdown)
	ccs := []EmailAddress{}
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
		To:        []EmailAddress{e.From},
		CC:        ccs,
		Subject:   "Re: " + e.Subject,
		Text:      text,
		HTML:      html,
	}
}
