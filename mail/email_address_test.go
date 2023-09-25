package mail_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/mail"
)

var _ = Describe("EmailAddress", func() {
	DescribeTable("Extracting names and addresses", func(input, name, address, tidy string) {
		email := mail.EmailAddress(input)
		Ω(email.Name()).Should(Equal(name))
		Ω(email.Address()).Should(Equal(address))
		Ω(email.String()).Should(Equal(tidy))
	},
		Entry(nil, "onsijoe@gmail.com", "onsijoe", "onsijoe@gmail.com", "onsijoe@gmail.com"),
		Entry(nil, "Onsi Fakhouri <onsijoe@gmail.com>", "Onsi", "onsijoe@gmail.com", "Onsi Fakhouri <onsijoe@gmail.com>"),
		Entry(nil, "Onsi Fakhouri <onsi.joe@gmail.com>", "Onsi", "onsi.joe@gmail.com", "Onsi Fakhouri <onsi.joe@gmail.com>"),
		Entry(nil, "Onsi Fakhouri <onsijoe+foo@gmail.com>", "Onsi", "onsijoe+foo@gmail.com", "Onsi Fakhouri <onsijoe+foo@gmail.com>"),
		Entry(nil, "Onsi Joe Salah Fakhouri <onsijoe@gmail.com>", "Onsi", "onsijoe@gmail.com", "Onsi Joe Salah Fakhouri <onsijoe@gmail.com>"),
		Entry(nil, "  Onsi Joe Salah  Fakhouri   <onsijoe@gmail.com>   ", "Onsi", "onsijoe@gmail.com", "Onsi Joe Salah  Fakhouri   <onsijoe@gmail.com>"),
		Entry(nil, "foo@example.com", "foo", "foo@example.com", "foo@example.com"),
		Entry(nil, "welp ", "welp", "welp", "welp"),
		Entry(nil, " wat@example.com <wat@who.com>", "wat@example.com", "wat@who.com", "wat@example.com <wat@who.com>"),
	)

	Describe("Comparing e-mail addresses", func() {
		It("considers e-mail addresses equal if the address portion is the same", func() {
			Ω(mail.EmailAddress("onsijoe@gmail.com").Equals("onsijoe@gmail.com")).To(BeTrue())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("onsijoe@gmail.com")).To(BeTrue())
			Ω(mail.EmailAddress("onsijoe@gmail.com").Equals("Onsi Fakhouri <onsijoe@gmail.com>")).To(BeTrue())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("Onesie Fakhouri <onsijoe@gmail.com>")).To(BeTrue())
			Ω(mail.EmailAddress("onsijoe+other@gmail.com").Equals("onsijoe@gmail.com")).To(BeFalse())
			Ω(mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>").Equals("Onsi Fakhouri <notonsijoe@gmail.com>")).To(BeFalse())
		})
	})
})
