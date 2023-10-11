package lunchtimedisco_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/lunchtimedisco"
)

var _ = Describe("LunchtimeParticipant", func() {
	Describe("updating game keys", func() {
		var lp lunchtimedisco.LunchtimeParticipants
		BeforeEach(func() {
			lp = lunchtimedisco.LunchtimeParticipants{
				{"player@example.com", []string{"A", "B", "D"}},
				{"Onsi Fakhouri <onsijoe@gmail.com>", []string{"A", "B", "C"}},
				{"anotherplayer@example.com", []string{"D", "F", "G"}},
			}
		})

		Describe("getting keys for a particular player", func() {
			It("returns the keys for that player", func() {
				Ω(lp.GamesFor("onsijoe@gmail.com")).Should(Equal("A,B,C"))
				Ω(lp.GamesFor("Onsi <onsijoe@gmail.com>")).Should(Equal("A,B,C"))
				Ω(lp.GamesFor("player@example.com")).Should(Equal("A,B,D"))
			})

			It("returns empty-string for non-existant players", func() {
				Ω(lp.GamesFor("nope")).Should(Equal(""))
			})
		})

		Describe("adding and updating participants", func() {
			It("can update participants", func() {
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"Jane <jane@example.com", []string{"A", "B", "C"}})
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"Onsi <onsijoe@gmail.com>", []string{"D"}})
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"anotherplayer@example.com", []string{}})
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"player@example.com", nil})
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"nope@example.com", nil})
				lp = lp.AddOrUpdate(lunchtimedisco.LunchtimeParticipant{"nope_again@example.com", []string{}})
				Ω(lp).Should(ConsistOf(
					lunchtimedisco.LunchtimeParticipant{"Jane <jane@example.com", []string{"A", "B", "C"}},
					lunchtimedisco.LunchtimeParticipant{"Onsi <onsijoe@gmail.com>", []string{"D"}},
				))
			})
		})
	})
})
