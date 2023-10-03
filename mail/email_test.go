package mail_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/mail"
	. "github.com/onsi/disco/mail"
)

var _ = Describe("Email", func() {
	It("can render a markdown formatted body", func() {
		email := Email{}
		email = email.WithBody(Markdown("# Hello\n\nThis is a *test*\n"))
		Ω(email.Text).Should(Equal("Hello\n\nThis is a test\n"))
		Ω(email.HTML).Should(Equal("<h1>Hello</h1>\n\n<p>This is a <em>test</em></p>\n"))
	})

	It("can render a plain text body", func() {
		email := Email{}
		email = email.WithBody("Hello\n\nThis is a *test*\n")
		Ω(email.Text).Should(Equal("Hello\n\nThis is a *test*\n"))
		Ω(email.HTML).Should(Equal(""))
	})

	It("has a convenient little DSL", func() {
		email := E().WithFrom("onsijoe@gmail.com").
			WithTo("player@example.com", "another@example.com").
			AndCC("onemore@example.com", "twomore@example.com").
			AndCC("threemore@example.com").
			WithSubject("Hello").
			WithBody(Markdown("This is a **test**"))

		Ω(email).Should(Equal(Email{
			From:    EmailAddress("onsijoe@gmail.com"),
			To:      EmailAddresses{"player@example.com", "another@example.com"},
			CC:      EmailAddresses{"onemore@example.com", "twomore@example.com", "threemore@example.com"},
			Subject: "Hello",
			Text:    "This is a test",
			HTML:    "<p>This is a <strong>test</strong></p>\n",
		}))
	})

	It("can return a concatenated list of recipients", func() {
		email := E().WithFrom("onsijoe@gmail.com").
			WithTo("player@example.com", "another@example.com").
			AndCC("onemore@example.com", "twomore@example.com").
			AndCC("threemore@example.com")
		Ω(email.Recipients()).Should(Equal(EmailAddresses{
			"player@example.com", "another@example.com",
			"onemore@example.com", "twomore@example.com",
			"threemore@example.com",
		}))
	})

	It("can test for recipients", func() {
		email := E().WithFrom("onsijoe@gmail.com").
			WithTo("player@example.com", "another@example.com").
			AndCC("onemore@example.com", "twomore@example.com").
			AndCC("threemore@example.com")
		Ω(email.IncludesRecipient(EmailAddress("onsijoe@gmail.com"))).Should(BeFalse())
		Ω(email.IncludesRecipient(EmailAddress("player@example.com"))).Should(BeTrue())
		Ω(email.IncludesRecipient(EmailAddress("Anne Other <another@example.com>"))).Should(BeTrue())
		Ω(email.IncludesRecipient(EmailAddress("threemore@example.com"))).Should(BeTrue())
		Ω(email.IncludesRecipient(EmailAddress("nope@nope.com"))).Should(BeFalse())
	})

	Describe("Replying to e-mails", func() {
		var email Email
		BeforeEach(func() {
			email = Email{
				MessageID: "<original-id>",
				From:      EmailAddress("onsijoe@gmail.com"),
				To:        EmailAddresses{"Disco <disco@sedenverultimate.net>", "list@googlegroups.com", "Onsi Fakhouri <onsijoe@gmail.com>"},
				CC:        EmailAddresses{"someone@else.com", "yet-another@someone.com", "Onsi <onsijoe@gmail.com>", "Disco Again <disco@sedenverultimate.net>"},
				Subject:   "Original Subject",
				Text:      "My **original** text\nIs _here_!",
				Date:      "Sun, 24 Sep 2023 13:48:58 -0600",
			}
		})

		It("can reply to an e-mail", func() {
			email = email.Reply("disco@sedenverultimate.net", Markdown("Got **your** message.\n\n_Thanks!_"))
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(BeEmpty())
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!\n\nOn Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n\n<div><blockquote type=\"cite\">On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:<br><br></blockquote></div>\n<blockquote type=\"cite\"><div>My **original** text<br>Is _here_!</div></blockquote>\n"))
		})

		It("doesn't double up the Res", func() {
			email = email.Reply("disco@sedenverultimate.net", "A").Reply("foo@example.com", "B")
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
		})

		It("can reply with plain text", func() {
			email = email.Reply("disco@sedenverultimate.net", "Got **your** message.\n\n_Thanks!_")
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(BeEmpty())
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got **your** message.\n\n_Thanks!_\n\nOn Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(BeZero())
		})

		It("can reply all to an e-mail", func() {
			email = email.ReplyAll("disco@sedenverultimate.net", Markdown("Got **your** message.\n\n_Thanks!_"))
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(ConsistOf(
				mail.EmailAddress("list@googlegroups.com"),
				mail.EmailAddress("someone@else.com"),
				mail.EmailAddress("yet-another@someone.com"),
				//note this does not include the replyer's e-mail address
			))
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!\n\nOn Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n\n<div><blockquote type=\"cite\">On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:<br><br></blockquote></div>\n<blockquote type=\"cite\"><div>My **original** text<br>Is _here_!</div></blockquote>\n"))
		})

		It("can reply without quoting the original e-mail", func() {
			email = email.ReplyWithoutQuote("disco@sedenverultimate.net", Markdown("Got **your** message.\n\n_Thanks!_"))
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(BeEmpty())
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n"))
		})

		It("can reply all without quoting the original email", func() {
			email = email.ReplyAllWithoutQuote("disco@sedenverultimate.net", Markdown("Got **your** message.\n\n_Thanks!_"))
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(Equal("<original-id>"))
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("onsijoe@gmail.com")))
			Ω(email.CC).Should(ConsistOf(
				mail.EmailAddress("list@googlegroups.com"),
				mail.EmailAddress("someone@else.com"),
				mail.EmailAddress("yet-another@someone.com"),
				//note this does not include the replyer's e-mail address
			))
			Ω(email.Subject).Should(Equal("Re: Original Subject"))
			Ω(email.Text).Should(Equal("Got your message.\n\nThanks!"))
			Ω(email.HTML).Should(Equal("<p>Got <strong>your</strong> message.</p>\n\n<p><em>Thanks!</em></p>\n"))
		})

		It("can forward an e-mail", func() {
			email = email.Forward("disco@sedenverultimate.net", "thirdparty@example.com", Markdown("Check **this** out."))
			Ω(email.MessageID).Should(BeZero())
			Ω(email.InReplyTo).Should(BeZero())
			Ω(email.From).Should(Equal(EmailAddress("disco@sedenverultimate.net")))
			Ω(email.To).Should(ConsistOf(EmailAddress("thirdparty@example.com")))
			Ω(email.CC).Should(BeEmpty())
			Ω(email.Subject).Should(Equal("Fwd: Original Subject"))
			Ω(email.Text).Should(Equal("Check this out.\n\nOn Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:\n> My **original** text\n> Is _here_!"))
			Ω(email.HTML).Should(Equal("<p>Check <strong>this</strong> out.</p>\n\n<div><blockquote type=\"cite\">On Sun, 24 Sep 2023 13:48:58 -0600, onsijoe@gmail.com wrote:<br><br></blockquote></div>\n<blockquote type=\"cite\"><div>My **original** text<br>Is _here_!</div></blockquote>\n"))
		})
	})
})
