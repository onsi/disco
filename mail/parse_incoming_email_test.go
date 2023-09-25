package mail_test

import (
	"os"

	"github.com/onsi/disco/mail"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func loadEmailFixture(filename string) []byte {
	GinkgoHelper()
	out, err := os.ReadFile("./fixtures/" + filename + ".json")
	Ω(err).ShouldNot(HaveOccurred())
	return out
}

var _ = Describe("ParseIncomingEmail", func() {
	It("extracts the key header pieces of information from an e-mail", func() {
		email, err := mail.ParseIncomingEmail(loadEmailFixture("email_from_ios"))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(email.From).Should(Equal(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
		Ω(email.To).Should(ConsistOf(mail.EmailAddress("saturday-disco@sedenverultimate.net")))
		Ω(email.CC).Should(ConsistOf(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
		Ω(email.Subject).Should(Equal("Hey Disco!"))
		Ω(email.InReplyTo).Should(BeZero())
		Ω(email.MessageID).Should(Equal("<C81E9CFE-81FC-477B-A3EA-1F6AB18870B4@gmail.com>"))
	})

	Context("when there are multiple to and CC recipients", func() {
		It("extracts them correctly", func() {
			email, err := mail.ParseIncomingEmail(loadEmailFixture("email_with_multiple_to_and_cc"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(email.From).Should(Equal(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
			Ω(email.To).Should(ConsistOf(mail.EmailAddress("saturday-disco@sedenverultimate.net"), mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
			Ω(email.CC).Should(ConsistOf(mail.EmailAddress("Onsi Fakhouri <onsijoe+foo@gmail.com>"), mail.EmailAddress("Onsi Fakhouri <onsijoe+bar@gmail.com>")))

			Ω(email.Subject).Should(Equal("Multiple Tos and CCs"))
			Ω(email.InReplyTo).Should(BeZero())
			Ω(email.MessageID).Should(Equal("<CAFmhaLZbzzxfNCkuqmC4vNY0wPtgJ=afHTdBwpEqJb2vuHXTug@mail.gmail.com>"))
		})
	})

	Describe("extracting bodies", func() {
		It("only extracts the text portion, ignoring HTML, and it grabs everything if this email is not a reply", func() {
			email, err := mail.ParseIncomingEmail(loadEmailFixture("email_from_ios"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(email.Text).Should(Equal("This is an email from iOS Mail.\n\nOnsi"))
			Ω(email.HTML).Should(BeZero())
		})

		Context("when the e-mail is a reply to a prior e-mail", func() {
			It("extracts just the text format of the most recent response, assuming it's on the top", func() {
				email, err := mail.ParseIncomingEmail(loadEmailFixture("reply_from_ios_mail"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(email.Text).Should(Equal("And this is my reply… from iOS Mail\n\n"))
				Ω(email.HTML).Should(BeZero())

				email, err = mail.ParseIncomingEmail(loadEmailFixture("reply_from_gmail_app"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(email.Text).Should(Equal("And this is another rely… from the *Gmail App*.\n\n"))
				Ω(email.HTML).Should(BeZero())
			})
		})
	})
})
