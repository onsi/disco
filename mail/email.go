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

type Email struct {
	MessageID string
	InReplyTo string

	From    EmailAddress
	To      []EmailAddress
	CC      []EmailAddress
	Subject string
	Date    string

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

func synthesizeReplyBodies(email Email, bodyMarkdown string) (string, string) {
	lines := strings.Split(email.Text, "\n")

	text := &strings.Builder{}
	text.WriteString(stripMarkdown(bodyMarkdown))
	text.WriteString("\n\n")
	fmt.Fprintf(text, "> On %s, %s wrote:\n\n", email.Date, email.From)
	for idx, line := range lines {
		text.WriteString("> ")
		text.WriteString(line)
		if idx < len(lines)-1 {
			text.WriteString("\n")
		}
	}

	html := &strings.Builder{}
	html.WriteString(renderMarkdown(bodyMarkdown))
	fmt.Fprintf(html, "\n<div><blockquote type=\"cite\">On %s, %s wrote:<br><br></blockquote></div>", stdlibhtml.EscapeString(email.Date), stdlibhtml.EscapeString(email.From.String()))
	html.WriteString("\n<blockquote type=\"cite\"><div>")
	for idx, line := range lines {
		html.WriteString(stdlibhtml.EscapeString(line))
		if idx < len(lines)-1 {
			html.WriteString("<br>")
		}
	}
	html.WriteString("</div></blockquote>\n")

	return text.String(), html.String()
}

func (e Email) WithBody(bodyMarkdown string) Email {
	e.Text = stripMarkdown(bodyMarkdown)
	e.HTML = renderMarkdown(bodyMarkdown)
	return e
}

func (e Email) Reply(from EmailAddress, bodyMarkdown string) Email {
	text, html := synthesizeReplyBodies(e, bodyMarkdown)
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
	text, html := synthesizeReplyBodies(e, bodyMarkdown)
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
