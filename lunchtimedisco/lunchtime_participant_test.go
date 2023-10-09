package lunchtimedisco_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
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

		Describe("clearing all keys (i.e. 'i can't play after all!')", func() {
			for _, clearCommand := range []string{"clear", "none", "no", "0"} {
				clearCommand := clearCommand
				Context("with the "+clearCommand+" variant", func() {
					It("drops players that match", func() {
						lp, message, err := lp.UpdateGameKeys("onsijoe@gmail.com", clearCommand)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(message).Should(Equal("cleared"))
						Ω(lp).Should(HaveLen(2))
						Ω(lp[0].Address.Address()).Should(Equal("player@example.com"))
						Ω(lp[0].GameKeys).Should(Equal([]string{"A", "B", "D"}))
						Ω(lp[1].Address.Address()).Should(Equal("anotherplayer@example.com"))
						Ω(lp[1].GameKeys).Should(Equal([]string{"D", "F", "G"}))
					})

					It("does nothing if no matching player is found", func() {
						lp2, message, err := lp.UpdateGameKeys("foo@example.com", clearCommand)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(message).Should(Equal("(nothing to clear)"))
						Ω(lp2).Should(Equal(lp))
					})
				})
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

		DescribeTable("updating game keys", func(input string, expectedError string, keys ...string) {
			lp2, message, err := lp.UpdateGameKeys("Onsi Fakhouri <onsijoe@gmail.com>", input)
			if expectedError == "" {
				Ω(err).ShouldNot(HaveOccurred())
				Ω(message).Should(Equal("updated to " + strings.Join(keys, ",")))
				Ω(lp2[1].GameKeys).Should(Equal(keys))
			} else {
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(ContainSubstring(expectedError))
			}

			lp3, message, err := lp.UpdateGameKeys("New Player <newplayer@example.com>", input)
			if expectedError == "" {
				Ω(err).ShouldNot(HaveOccurred())
				Ω(message).Should(Equal("set to " + strings.Join(keys, ",")))
				Ω(lp3[3].Address).Should(Equal(mail.EmailAddress("New Player <newplayer@example.com>")))
				Ω(lp3[3].GameKeys).Should(Equal(keys))
			} else {
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(ContainSubstring(expectedError))
			}

		},
			Entry(nil, "A", "", "A"),
			Entry(nil, "A, B", "", "A", "B"),
			Entry(nil, "  A , B   , C ", "", "A", "B", "C"),
			Entry(nil, "a,b,c,!C", "", "A", "B"),
			Entry(nil, "A,B-E,H", "", "A", "B", "C", "D", "E", "H"),
			Entry(nil, "A,B-E,H,B,C,D,A", "", "A", "B", "C", "D", "E", "H"),
			Entry(nil, "A,B-E,H,!D", "", "A", "B", "C", "E", "H"),
			Entry(nil, "all", "", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"),
			Entry(nil, "all,!C", "", "A", "B", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"),
			Entry(nil, "all,!D-G, !L", "", "A", "B", "C", "H", "I", "J", "K", "M", "N", "O", "P"),
			Entry(nil, "A-A", "", "A"),
			Entry(nil, "A-G,f - k, ! h-i", "", "A", "B", "C", "D", "E", "F", "G", "J", "K"),
			Entry(nil, "foo", "FOO is not a valid game-key"),
			Entry(nil, "A,foo", "FOO is not a valid game-key"),
			Entry(nil, "M-Q", "M-Q is not a valid game-key"),
			Entry(nil, "A,!A", "no game-keys were left"),
		)
	})
})
