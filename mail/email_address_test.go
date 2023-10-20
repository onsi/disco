package mail_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/mail"
)

var _ = Describe("EmailAddress", func() {
	DescribeTable("Extracting names and addresses", func(input, name, address, tidy string, hasExplicitName bool) {
		email := mail.EmailAddress(input)
		Ω(email.Name()).Should(Equal(name))
		Ω(email.Address()).Should(Equal(address))
		Ω(email.String()).Should(Equal(tidy))
		Ω(email.HasExplicitName()).Should(Equal(hasExplicitName))
	},
		Entry(nil, "onsijoe@gmail.com", "onsijoe", "onsijoe@gmail.com", "onsijoe@gmail.com", false),
		Entry(nil, "Onsi Fakhouri <onsijoe@gmail.com>", "Onsi", "onsijoe@gmail.com", "Onsi Fakhouri <onsijoe@gmail.com>", true),
		Entry(nil, "Onsi Fakhouri <onsi.joe@gmail.com>", "Onsi", "onsi.joe@gmail.com", "Onsi Fakhouri <onsi.joe@gmail.com>", true),
		Entry(nil, "Onsi Fakhouri <onsijoe+foo@gmail.com>", "Onsi", "onsijoe+foo@gmail.com", "Onsi Fakhouri <onsijoe+foo@gmail.com>", true),
		Entry(nil, "Onsi Joe Salah Fakhouri <onsijoe@gmail.com>", "Onsi", "onsijoe@gmail.com", "Onsi Joe Salah Fakhouri <onsijoe@gmail.com>", true),
		Entry(nil, "  Onsi Joe Salah  Fakhouri   <onsijoe@gmail.com>   ", "Onsi", "onsijoe@gmail.com", "Onsi Joe Salah  Fakhouri   <onsijoe@gmail.com>", true),
		Entry(nil, "  onsi fakhouri   <onsijoe@gmail.com>   ", "Onsi", "onsijoe@gmail.com", "onsi fakhouri   <onsijoe@gmail.com>", true),
		Entry(nil, "foo@example.com", "foo", "foo@example.com", "foo@example.com", false),
		Entry(nil, "welp ", "welp", "welp", "welp", false),
		Entry(nil, " wat@example.com <wat@who.com>", "Wat@Example.Com", "wat@who.com", "wat@example.com <wat@who.com>", true),
	)

	Describe("Comparing e-mail addresses", func() {
		It("considers e-mail addresses equal if the address portion is the same", func() {
			Ω(mail.EmailAddress("onsijoe@gmail.com").Equals("onsijoe@gmail.com")).To(BeTrue())
			Ω(mail.EmailAddress("onsijoe@gmail.com").Equals("OnsiJoe@gmail.com")).To(BeTrue())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("onsijoe@gmail.com")).To(BeTrue())
			Ω(mail.EmailAddress("onsijoe@gmail.com").Equals("Onsi Fakhouri <onsijoe@gmail.com>")).To(BeTrue())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("Onesie Fakhouri <onsijoe@gmail.com>")).To(BeTrue())
			Ω(mail.EmailAddress("onsijoe+other@gmail.com").Equals("onsijoe@gmail.com")).To(BeFalse())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("Onsi Fakhouri <notonsijoe@gmail.com>")).To(BeFalse())
		})
	})

	It("can stringify a bunch of email addresses", func() {
		addresses := mail.EmailAddresses{"Onsi Fakhouri <onsijoe@gmail.com>", "foo@example.com"}
		Ω(addresses.String()).Should(Equal("Onsi Fakhouri <onsijoe@gmail.com>, foo@example.com"))
	})
})
