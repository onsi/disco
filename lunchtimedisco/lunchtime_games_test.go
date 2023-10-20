package lunchtimedisco_test

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clockpkg "github.com/onsi/disco/clock"
	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/weather"
)

var _ = Describe("LunchtimeGames", func() {
	var address1, address2, address3 mail.EmailAddress
	BeforeEach(func() {
		address1 = mail.EmailAddress("player1@example.com")
		address2 = mail.EmailAddress("player2@example.com")
		address3 = mail.EmailAddress("player3@example.com")
	})

	Describe("Game", func() {
		It("knows when its zero", func() {
			Ω(lunchtimedisco.Game{}).Should(BeZero())
			Ω(lunchtimedisco.Game{Key: "A"}).ShouldNot(BeZero())
		})

		It("can return a count", func() {
			Ω(lunchtimedisco.Game{}.Count()).Should(Equal(0))
			Ω(lunchtimedisco.Game{
				Players: mail.EmailAddresses{
					"onsijoe@gmail.com",
					"player@example.com",
					"anotherplayer@example.com",
				},
			}.Count()).Should(Equal(3))
		})

		It("can return a public list of players", func() {
			Ω(lunchtimedisco.Game{}.PublicParticipants()).Should(Equal("No one's signed up yet"))
			Ω(lunchtimedisco.Game{
				Players: mail.EmailAddresses{
					"Onsi Fakhouri <onsijoe@gmail.com>",
					"yoyoma@cello.com",
					"player@example.com",
				},
			}.PublicParticipants()).Should(Equal("Onsi, yoyoma and player"))
		})
	})

	Describe("Games", func() {
		It("can return a particular game", func() {
			gameLookup := map[string]lunchtimedisco.Game{}
			games := lunchtimedisco.Games{}
			for _, key := range lunchtimedisco.GameKeys {
				game := lunchtimedisco.Game{
					Key: key,
					Players: mail.EmailAddresses{
						mail.EmailAddress(key + "@example.com"),
						mail.EmailAddress(key + "gmail.com"),
					},
				}
				gameLookup[key] = game
				games = append(games, game)
			}
			for _, key := range lunchtimedisco.GameKeys {
				Ω(reflect.ValueOf(games).MethodByName(key).Call([]reflect.Value{})[0].Interface()).Should(Equal(gameLookup[key]))
				Ω(games.Game(key)).Should(Equal(gameLookup[key]))
			}
		})
	})

	Describe("building games", func() {
		var forecaster *weather.FakeForecaster
		var T time.Time
		var participants lunchtimedisco.LunchtimeParticipants
		var games lunchtimedisco.Games
		var tuesdayAt10 time.Time
		BeforeEach(func() {
			forecaster = weather.NewFakeForecaster()
			forecaster.SetForecast(weather.Forecast{Temperature: 72})
			now := time.Date(2023, time.September, 24, 0, 0, 0, 0, clockpkg.Timezone) // a Sunday
			T = clockpkg.NextSaturdayAt10(now)
			participants = lunchtimedisco.LunchtimeParticipants{
				{Address: address1, GameKeys: []string{"A", "B", "C", "G", "I", "O"}},
				{Address: address2, GameKeys: []string{"C", "D", "E", "F", "G", "H", "P"}},
				{Address: address3, GameKeys: []string{"G", "H", "I", "J", "M"}},
				{Address: mail.EmailAddress("onsijoe@gmail.com"), GameKeys: []string{}},
			}
			games = lunchtimedisco.BuildGames(GinkgoWriter, T, participants, forecaster)
			tuesdayAt10 = T.Add(-4 * 24 * time.Hour)
			Ω(tuesdayAt10.Weekday()).Should(Equal(time.Tuesday))
			Ω(tuesdayAt10.Hour()).Should(Equal(10))
		})

		It("builds games with appropriate start times and weather forecasts", func() {
			G := func(key string, dt int, players ...mail.EmailAddress) lunchtimedisco.Game {
				dtHour := time.Duration(dt) * time.Hour
				if players == nil {
					players = mail.EmailAddresses{}
				}
				return lunchtimedisco.Game{
					Key:       key,
					Players:   players,
					StartTime: tuesdayAt10.Add(dtHour),
					Forecast: weather.Forecast{
						Temperature: 72,
						StartTime:   tuesdayAt10.Add(dtHour),
						EndTime:     tuesdayAt10.Add(dtHour + time.Hour),
					},
				}
			}

			Ω(games.A()).Should(Equal(G("A", 0, address1)))
			Ω(games.B()).Should(Equal(G("B", 1, address1)))
			Ω(games.C()).Should(Equal(G("C", 2, address1, address2)))
			Ω(games.D()).Should(Equal(G("D", 3, address2)))
			Ω(games.E()).Should(Equal(G("E", 24, address2)))
			Ω(games.F()).Should(Equal(G("F", 25, address2)))
			Ω(games.G()).Should(Equal(G("G", 26, address1, address2, address3)))
			Ω(games.H()).Should(Equal(G("H", 27, address2, address3)))
			Ω(games.I()).Should(Equal(G("I", 48, address1, address3)))
			Ω(games.J()).Should(Equal(G("J", 49, address3)))
			Ω(games.K()).Should(Equal(G("K", 50)))
			Ω(games.L()).Should(Equal(G("L", 51)))
			Ω(games.M()).Should(Equal(G("M", 72, address3)))
			Ω(games.N()).Should(Equal(G("N", 73)))
			Ω(games.O()).Should(Equal(G("O", 74, address1)))
			Ω(games.P()).Should(Equal(G("P", 75, address2)))
		})
	})
})
