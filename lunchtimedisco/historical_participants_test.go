package lunchtimedisco_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
)

var _ = Describe("HistoricalParticipants", func() {
	It("maintains a list of historical participants - saving off the latest e-mail but not duplicating emails", func() {
		hp := lunchtimedisco.HistoricalParticipants{}

		hp = hp.AddOrUpdate("onsijoe@gmail.com")
		Ω(hp).Should(ConsistOf(mail.EmailAddress("onsijoe@gmail.com")))

		hp = hp.AddOrUpdate("jane@example.com")
		hp = hp.AddOrUpdate("jane@example.com")
		Ω(hp).Should(ConsistOf(
			mail.EmailAddress("onsijoe@gmail.com"),
			mail.EmailAddress("jane@example.com")))

		hp = hp.AddOrUpdate("Onsi Fakhouri <onsijoe@gmail.com>")
		Ω(hp).Should(ConsistOf(
			mail.EmailAddress("Onsi Fakhouri <onsijoe@gmail.com>"),
			mail.EmailAddress("jane@example.com")))
	})
})
