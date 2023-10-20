package lunchtimedisco_test

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clockpkg "github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/server"
	"github.com/onsi/disco/weather"

	. "github.com/onsi/disco/lunchtimedisco"
)

var _ = Describe("LunchtimeDisco", func() {
	var outbox *mail.FakeOutbox
	var clock *clockpkg.FakeAlarmClock
	var forecaster *weather.FakeForecaster
	var disco *LunchtimeDisco
	var db *s3db.FakeS3DB
	var conf config.Config

	var now time.Time
	var weekOf string
	var playerEmail mail.EmailAddress
	var playerName string
	var le func() mail.Email
	var homeURL, playerURL, persistentPlayerURL, bossURL string
	var forecast weather.Forecast

	signUpPlayer := func(name string, email string, comments string, gameKeys []string) {
		GinkgoHelper()
		b.Navigate(playerURL)
		Eventually("#name").Should(b.SetValue(name))
		Ω("#email").Should(b.SetValue(email))
		Eventually(".validation-error").ShouldNot(b.Exist())
		if comments != "" {
			Ω("#comments").Should(b.SetValue(comments))
		}
		for _, key := range gameKeys {
			Ω("#" + key).Should(b.Click())
			Eventually("#" + key).Should(b.HaveClass("selected"))
		}
		Ω(".submit").Should(b.Click())
		Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 1))
	}

	sendInvite := func() {
		GinkgoHelper()
		//first we get the monitor e-mail
		clock.Fire()
		Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
		outbox.Clear()

		//then we send the invite
		b.Navigate(bossURL)
		Eventually("#invite").Should(b.Click())
		Eventually(b.XPath("button").WithText("Send Invite")).Should(b.Click())
		Eventually(le).Should(HaveSubject("Lunchtime Bible Park Frisbee - Week of " + weekOf))
		outbox.Clear()
		Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
	}

	BeforeEach(func() {
		outbox = mail.NewFakeOutbox()
		le = outbox.LastEmail
		clock = clockpkg.NewFakeAlarmClock()
		forecaster = weather.NewFakeForecaster()
		forecast = weather.Forecast{
			StartTime:                  time.Now(), // so we're non-zero
			Temperature:                72,
			TemperatureUnit:            "F",
			WindSpeed:                  "8 mph",
			ProbabilityOfPrecipitation: 10,
			ShortForecast:              "Partly Cloud",
			ShortForecastEmoji:         "🌤️",
		}
		forecaster.SetForecast(forecast)
		db = s3db.NewFakeS3DB()
		conf.BossEmail = mail.EmailAddress("Boss <boss@example.com>")
		conf.LunchtimeDiscoEmail = mail.EmailAddress("Disco <lunchtime-disco@sedenverultimate.net>")
		conf.LunchtimeDiscoList = mail.EmailAddress("southeast-denver-lunchtime-ultimate@googlegroups.com")
		playerEmail = mail.EmailAddress("John Player <player@example.com>")
		playerName = "John Player"

		now = time.Date(2023, time.September, 24, 0, 0, 0, 0, clockpkg.Timezone) // a Sunday
		clock.SetTime(now)
		weekOf = "9/25"

		var err error
		disco, err = NewLunchtimeDisco(conf, GinkgoWriter, clock, outbox, forecaster, db)
		Ω(err).ShouldNot(HaveOccurred())
		DeferCleanup(disco.Stop)
		Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
		outbox.Clear() //clear out the welcome email

		conf.Port = fmt.Sprintf("99%02d", GinkgoParallelProcess())
		e := echo.New()
		e.Logger.SetOutput(GinkgoWriter)
		s := server.NewServer(e, "../", conf, outbox, db, nil, disco)
		go s.Start()
		DeferCleanup(e.Shutdown, NodeTimeout(time.Second))

		homeURL = fmt.Sprintf("http://localhost:%s", conf.Port)
		persistentPlayerURL = fmt.Sprintf("%s/lunchtime/%s", homeURL, disco.GUID)
		playerURL = fmt.Sprintf("%s/lunchtime/%s?reset", homeURL, disco.GUID)
		bossURL = fmt.Sprintf("%s/lunchtime/%s", homeURL, disco.BossGUID)
		Eventually(http.Get).WithArguments(homeURL).Should(HaveField("StatusCode", http.StatusOK))
	})

	Describe("startup stuff", func() {

	})

	Describe("the scheduler", func() {
		It("sends the boss a daily ping", func() {
			clock.Fire()
			Ω(clock.Time()).Should(BeOn(time.Sunday, 6))
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			Ω(le()).Should(BeFrom(conf.LunchtimeDiscoEmail))
			Ω(le()).Should(BeSentTo(conf.BossEmail))
			Ω(le()).Should(HaveText(ContainSubstring("Here's the latest on the lunchtime game.")))
			Ω(le()).Should(HaveText(ContainSubstring("Dashboard: https://www.sedenverultimate.net/lunchtime/" + disco.GetSnapshot().BossGUID)))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 0 - Tuesday 9/26 at 10:00am - %s\n+ B - 0 - Tuesday 9/26 at 11:00am - %s", forecast, forecast)))
			outbox.Clear()

			signUpPlayer(playerName, playerEmail.Address(), "", []string{"A"})
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 1))

			clock.Fire()
			Ω(clock.Time()).Should(BeOn(time.Monday, 6))
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 1 - Tuesday 9/26 at 10:00am - %s\n  - John Player <player@example.com>\n+ B - 0 - Tuesday 9/26 at 11:00am - %s\n+ C", forecast, forecast)))
			outbox.Clear()

			signUpPlayer("Bob Player", "bob@example.com", "", []string{"A", "B"})
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 2))

			clock.Fire()
			Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 2 - Tuesday 9/26 at 10:00am - %s\n  - John Player <player@example.com>\n  - Bob Player <bob@example.com>\n+ B - 1 - Tuesday 9/26 at 11:00am - %s\n  - Bob Player <bob@example.com>\n+ C", forecast, forecast)))
		})

		Context("when the invite is sent", func() {
			BeforeEach(func() {
				sendInvite()
			})

			It("keeps sending the daily ping", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Monday, 6))
				Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
				Ω(le()).Should(BeFrom(conf.LunchtimeDiscoEmail))
				Ω(le()).Should(BeSentTo(conf.BossEmail))
				outbox.Clear()

				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
				Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			})
		})

		Context("when no-invite is sent", func() {

			It("stops sending the daily ping ", func() {

			})

			It("eventually resets", func() {

			})
		})

		Context("when no-game is called", func() {
			It("stops sending the daily ping", func() {

			})

			It("eventually resets", func() {

			})
		})

		Context("when game is called", func() {
			It("stops sending the daily ping", func() {

			})

			It("sends a reminder instead", func() {

			})

			Context("after the reminder is sent", func() {
				It("eventually resets", func() {

				})
			})
		})
	})

	Describe("attaching to the email thread", func() {
		It("sends the invite to a new thread but then all subsequent e-mails to that thread", func() {

		})
	})

	Describe("allowing players to sign up", func() {

	})

	Describe("allowing the boss to see who has signed up (via email)", func() {

	})

	Describe("allowing the boss to see who has signed up (via web)", func() {
		//test clicking on the name pills
		//test viewing the table
	})

	Describe("allowing the boss to modify sign-ups", func() {
		//test historical participants
	})

	Describe("boss sending invite", func() {

	})

	Describe("boss sending no-invite", func() {

	})

	Describe("boss sending a badger", func() {

	})

	Describe("boss calling game", func() {
		//don't forget to test front page
		Context("with the standard time", func() {

		})

		Context("with a time override", func() {

		})
	})

	Describe("boss calling no game", func() {
		//don't forget to test the front page
	})

	It("works", func() {
		fmt.Fprint(io.Discard, playerEmail, le(), playerURL, bossURL, persistentPlayerURL)
		b.Navigate(homeURL)
		Eventually("#content").Should(b.Exist())
	})
})
