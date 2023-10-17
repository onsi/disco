package mail_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
)

var _ = Describe("Outbox", func() {
	var outbox mail.OutboxInt
	BeforeEach(func() {
		if os.Getenv("TEST_OUTBOX") != "true" {
			Skip("Skipping outbox tests, set TEST_OUTBOX=true to run")
		}
		c := config.LoadConfig()
		outbox = mail.NewOutbox(c.ForwardEmailKey, c.GmailUser, c.GmailPassword)
	})

	It("can send a multipart e-mail from onsi's gmail account", func() {
		m := mail.E().WithFrom("Onsi Fakhouri <onsijoe@gmail.com>").
			WithTo("Onsi Fakhouri <onsijoe@gmail.com>").
			AndCC("Onsi Fakhouri <onsijoe@outlook.com>").
			WithSubject("Test e-mail (gmail)").WithBody(mail.Markdown("# Hello there\n\nThis is a **test e-mail**!"))
		Ω(outbox.SendEmail(m)).Should(Succeed())
	})

	It("can send a multipart e-mail from forwardemail", func() {
		m := mail.E().WithFrom("Saturday Disco <saturday-disco@sedenverultimate.net>").
			WithTo("Onsi Fakhouri <onsijoe@gmail.com>").
			AndCC("Onsi Fakhouri <onsijoe@outlook.com>").
			WithSubject("Test e-mail (forwardemail)").WithBody(mail.Markdown("# Hello there\n\nThis is a **test e-mail**!"))
		Ω(outbox.SendEmail(m)).Should(Succeed())
	})

	It("fails for unknown email addresses", func() {
		m := mail.E().WithFrom("example@gmail.com").
			WithTo("Onsi Fakhouri <onsijoe@gmail.com>").
			AndCC("Onsi Fakhouri <onsijoe@outlook.com>").
			WithSubject("Test e-mail (fail)").WithBody(mail.Markdown("# Hello there\n\nThis is a **test e-mail**!"))
		Ω(outbox.SendEmail(m)).Should(HaveOccurred())
	})
})
