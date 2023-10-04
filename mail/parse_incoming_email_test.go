package mail_test

import (
	"os"

	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func loadEmailFixture(filename string) []byte {
	GinkgoHelper()
	out, err := os.ReadFile("./fixtures/" + filename)
	Ω(err).ShouldNot(HaveOccurred())
	return out
}

var _ = Describe("ParseIncomingEmail", func() {
	var db *s3db.FakeS3DB
	BeforeEach(func() {
		db = s3db.NewFakeS3DB()
	})

	It("extracts the key header pieces of information from an e-mail", func() {
		email, err := mail.ParseIncomingEmail(db, loadEmailFixture("email_from_ios.json"), GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(email.From).Should(Equal(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
		Ω(email.To).Should(ConsistOf(mail.EmailAddress("saturday-disco@sedenverultimate.net")))
		Ω(email.CC).Should(ConsistOf(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
		Ω(email.Subject).Should(Equal("Hey Disco!"))
		Ω(email.InReplyTo).Should(BeZero())
		Ω(email.MessageID).Should(Equal("<C81E9CFE-81FC-477B-A3EA-1F6AB18870B4@gmail.com>"))
		Ω(email.Date).Should(Equal("Sat, 23 Sep 2023 16:47:41 -0600"))
	})

	It("stores the raw version of the email in the db for future debugging", func() {
		rawEmail := loadEmailFixture("email_from_ios.json")
		email, err := mail.ParseIncomingEmail(db, rawEmail, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())
		Eventually(db.FetchObject).WithArguments(email.DebugKey).Should(Equal(rawEmail))
	})

	Context("when there are multiple to and CC recipients", func() {
		It("extracts them correctly", func() {
			email, err := mail.ParseIncomingEmail(db, loadEmailFixture("email_with_multiple_to_and_cc.json"), GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(email.From).Should(Equal(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
			Ω(email.To).Should(ConsistOf(mail.EmailAddress("saturday-disco@sedenverultimate.net"), mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>")))
			Ω(email.CC).Should(ConsistOf(mail.EmailAddress("Onsi Fakhouri <onsijoe+foo@gmail.com>"), mail.EmailAddress("Onsi Fakhouri <onsijoe+bar@gmail.com>")))

			Ω(email.Subject).Should(Equal("Multiple Tos and CCs"))
			Ω(email.InReplyTo).Should(BeZero())
			Ω(email.MessageID).Should(Equal("<CAFmhaLZbzzxfNCkuqmC4vNY0wPtgJ=afHTdBwpEqJb2vuHXTug@mail.gmail.com>"))
			Ω(email.Date).Should(Equal("Sun, 24 Sep 2023 13:48:58 -0600"))
		})
	})

	Describe("handling mailing list e-mails", func() {
		Context("when a user replies all and we get two e-mails - one to the list and one to disco", func() {
			It("extracts the actual e-mail address not the weird 'via mailing list' email address. also we spotcheck the real-life fixtures to ensure the IDs are the same", func() {
				all, err := mail.ParseIncomingEmail(db, loadEmailFixture("mailing_list_reply_all.email"), GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(all.From).Should(Equal(mail.EmailAddress("Redacted Player <redactedplayer@yahoo.com>")))

				toDisco, err := mail.ParseIncomingEmail(db, loadEmailFixture("mailing_list_reply_disco.email"), GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(all.MessageID).Should(Equal(toDisco.MessageID), "These e-mails represent the same e-mail")
			})
		})
	})

	Describe("extracting bodies", func() {
		It("only extracts the text portion, ignoring HTML, and it grabs everything if this email is not a reply", func() {
			email, err := mail.ParseIncomingEmail(db, loadEmailFixture("email_from_ios.json"), GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(email.Text).Should(Equal("This is an email from iOS Mail.\n\nOnsi"))
			Ω(email.HTML).Should(BeZero())
		})

		Context("when the e-mail is a reply to a prior e-mail", func() {
			It("extracts just the text format of the most recent response, assuming it's on the top", func() {
				email, err := mail.ParseIncomingEmail(db, loadEmailFixture("reply_from_ios_mail.json"), GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(email.Text).Should(Equal("And this is my reply… from iOS Mail\n\n"))
				Ω(email.HTML).Should(BeZero())

				email, err = mail.ParseIncomingEmail(db, loadEmailFixture("reply_from_gmail_app.json"), GinkgoWriter)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(email.Text).Should(Equal("And this is another rely… from the *Gmail App*.\n"))
				Ω(email.HTML).Should(BeZero())
			})
		})
		Context("when the e-mail is a forward", func() {
			It("correctly extracts the body", func() {
				fullBody := loadEmailFixture("forward_body.email")
				Ω(mail.ExtractTopMostPortion(string(fullBody))).Should(Equal("/set Redacted Redacted <redacted.r.redacted@gmail.com> 2\n"))
			})
		})

		Context("when the e-mail is a reply", func() {
			It("correctly extracts the body", func() {
				fullBody := loadEmailFixture("real_reply.email")
				Ω(mail.ExtractTopMostPortion(string(fullBody))).Should(Equal("I'm in\n"))
				fullBody = loadEmailFixture("another_real_reply.email")
				Ω(mail.ExtractTopMostPortion(string(fullBody))).Should(Equal(" I'm in - bringing REDACTED as well.\n"))
			})
		})
	})
})
