package mail_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/onsi/disco/mail"
)

var _ = Describe("Email", func() {
	It("can render a markdown formatted body", func() {
		email := mail.Email{}
		email = email.WithBody("# Hello\n\nThis is a *test*\n")
		Ω(email.Text).Should(Equal("Hello\n\nThis is a test\n"))
		Ω(email.HTML).Should(Equal("<h1>Hello</h1>\n\n<p>This is a <em>test</em></p>\n"))
	})

	Describe("Replying to e-mails", func() {
		var email mail.Email
		BeforeEach(func() {
			email = mail.Email{
				MessageID: "<original-id>",
				From:      mail.EmailAddress("onsijoe@gmail.com"),
				To:        []mail.EmailAddress{"Disco <disco@sedenverultimate.net>", "list@googlegroups.com", "Onsi Fakhouri <onsijoe@gmail.com>"},
				CC:        []mail.EmailAddress{"someone@else.com", "yet-another@someone.com", "Onsi <onsijoe@gmail.com>", "Disco Again <disco@sedenverultimate.net>"},
				Subject:   "Original Subject",
				Text:      "My **original** text\nIs _here_!",
				Date:      "Sun, 24 Sep 2023 13:48:58 -0600",
			}
		})

		It("can reply to an e-mail", func() {
			format.TruncatedDiff = false
			email = email.Reply("disco@sedenverultimate.net", "Got **your** message.\n\n_Thanks!_")
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(mail.EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(mail.EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(BeEmpty())
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!\n\n> On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n\n<div><blockquote type=\"cite\">On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:<br><br></blockquote></div>\n<blockquote type=\"cite\"><div>My **original** text<br>Is _here_!</div></blockquote>\n"))
		})

		It("can reply all to an e-mail", func() {
			format.TruncatedDiff = false
			email = email.ReplyAll("disco@sedenverultimate.net", "Got **your** message.\n\n_Thanks!_")
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(mail.EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(mail.EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(ConsistOf(
				mail.EmailAddress("list@googlegroups.com"),
				mail.EmailAddress("someone@else.com"),
				mail.EmailAddress("yet-another@someone.com"),
				//note this does not include the replyer's e-mail address
			))
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!\n\n> On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n\n<div><blockquote type=\"cite\">On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:<br><br></blockquote></div>\n<blockquote type=\"cite\"><div>My **original** text<br>Is _here_!</div></blockquote>\n"))
		})
	})
})
