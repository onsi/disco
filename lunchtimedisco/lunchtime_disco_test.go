package lunchtimedisco_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clockpkg "github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/lunchtimedisco"
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
	var indexURL, playerURL, persistentPlayerURL, bossURL string
	var forecast weather.Forecast

	signUpPlayer := func(name string, email string, comments string, gameKeys []string) {
		GinkgoHelper()
		b.Navigate(playerURL)
		Eventually("#name").Should(b.SetValue(name))
		Ω("#email").Should(b.SetValue(email))
		Eventually(".validation-error").ShouldNot(b.Exist())
		Ω("#comments").Should(b.SetValue(comments))
		selected := b.GetPropertyForEach(".selected", "id")
		for _, key := range selected {
			Ω("#" + key.(string)).Should(b.Click())
		}
		for _, key := range gameKeys {
			Ω("#" + key).Should(b.Click())
			Eventually("#" + key).Should(b.HaveClass("selected"))
		}
		Ω(".submit").Should(b.Click())
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
		Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Invite")).Should(b.Click())
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
		DeferCleanup(e.Shutdown, NodeTimeout(10*time.Second))

		indexURL = fmt.Sprintf("http://localhost:%s", conf.Port)
		persistentPlayerURL = fmt.Sprintf("%s/lunchtime/%s", indexURL, disco.GUID)
		playerURL = fmt.Sprintf("%s/lunchtime/%s?reset", indexURL, disco.GUID)
		bossURL = fmt.Sprintf("%s/lunchtime/%s", indexURL, disco.BossGUID)
		Eventually(http.Get).WithArguments(indexURL).Should(HaveField("StatusCode", http.StatusOK))
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
			BeforeEach(func() {
				//first we get the monitor e-mail
				clock.Fire()
				Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
				outbox.Clear()

				//then we send the invite
				b.Navigate(bossURL)
				Eventually("#no-invite").Should(b.Click())
				Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Invite")).Should(b.Click())
				Eventually(le).Should(HaveSubject("No Lunchtime Bible Park Frisbee This Week"))
				outbox.Clear()
				Ω(disco.GetSnapshot()).Should(HaveState(StateNoInviteSent))
			})

			It("stops sending the daily ping and, instead, resets the following Saturday", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
				Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
				Ω(le()).Should(BeZero())
			})
		})

		Context("when no-game is called", func() {
			BeforeEach(func() {
				sendInvite()
				b.Navigate(bossURL)
				Eventually("#no-game").Should(b.Click())
				Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Game")).Should(b.Click())
				Eventually(le).Should(HaveSubject("No Lunchtime Game This Week"))
				outbox.Clear()

				Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
			})

			It("stops sending the daily ping and, instead, resets the following Saturday", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
				Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
				Ω(le()).Should(BeZero())
			})
		})

		Context("when game is called", func() {
			BeforeEach(func() {
				sendInvite()
				b.Navigate(bossURL)
				Eventually("#game-on").Should(b.Click())
				Eventually("#game-option-E").Should(b.Click())
				Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Game On")).Should(b.Click())
				Eventually(le).Should(HaveSubject("GAME ON! Wednesday 9/27 at 10:00am"))
				outbox.Clear()

				Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
			})

			It("stops sending the daily ping and sends a reminder instead and eventually resets", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Wednesday, 6))
				Eventually(disco.GetSnapshot).Should(HaveState(StateReminderSent))
				Ω(le()).Should(HaveSubject("Reminder: GAME ON TODAY! Wednesday 9/27 at 10:00am"))
				outbox.Clear()

				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
				Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
				Ω(le()).Should(BeZero())
			})
		})
	})

	Describe("attaching to the email thread", func() {
		It("sends the invite to a new thread but then all subsequent e-mails to that thread", func() {
			//first we get the monitor e-mail
			clock.Fire()
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			outbox.Clear()

			//then we send the invite
			b.Navigate(bossURL)
			Eventually("#invite").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Lunchtime Bible Park Frisbee - Week of " + weekOf))
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			threadId := le().MessageID
			Ω(threadId).ShouldNot(BeZero())

			//now we process the invite, which is what will happen when we, as a member of the list receive it
			disco.HandleIncomingEmail(le())
			outbox.Clear()
			Eventually(disco.GetSnapshot).Should(HaveField("ThreadEmail.IsZero()", BeFalse()))

			//note that we don't get confused if another e-mail comes in
			email := mail.E().WithFrom(playerEmail).WithTo(conf.LunchtimeDiscoList).WithSubject("Hey there").WithBody("I'm in!")
			email.MessageID = "DECOY"
			disco.HandleIncomingEmail(email)
			Consistently(disco.GetSnapshot).Should(HaveField("ThreadEmail.MessageID", Equal(threadId)))

			//even if it's an e-mail from the boss
			email = mail.E().WithFrom(conf.BossEmail).WithTo(conf.LunchtimeDiscoList).WithSubject("Hey there").WithBody("I'm in!")
			email.MessageID = "BOSS_DECOY"
			disco.HandleIncomingEmail(email)
			Consistently(disco.GetSnapshot).Should(HaveField("ThreadEmail.MessageID", Equal(threadId)))

			//now subsequent e-mails should be sent to the same thread (and not a decoy/other thread!)
			b.Navigate(bossURL)
			Eventually("#game-on").Should(b.Click())
			Eventually("#game-option-E").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Game On")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Re: Lunchtime Bible Park Frisbee - Week of " + weekOf))
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le().InReplyTo).Should(Equal(threadId))
			Ω(le()).Should(HaveText(ContainSubstring("GAME ON for Wednesday 9/27 at 10:00am")))
		})
	})

	Describe("preventing access", func() {
		It("returns 404 if someone without the magic guid tries to access", func() {
			Ω(http.Get(indexURL + "/lunchtime")).Should(HaveHTTPStatus(http.StatusNotFound))
			Ω(http.Get(indexURL + "/lunchtime/HACK-GUID")).Should(HaveHTTPStatus(http.StatusNotFound))
			badger, err := json.Marshal(lunchtimedisco.Command{
				CommandType: lunchtimedisco.CommandAdminBadger,
			})
			buf := bytes.NewBuffer(badger)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(http.Post(indexURL+"/lunchtime/HACK-GUID", "application/json", buf)).Should(HaveHTTPStatus(http.StatusNotFound))
		})
	})

	Describe("allowing players to sign up", func() {
		It("validates that the user enters a name and an e-mail address, and records their selection and any comments they make", func() {
			sendInvite()

			By("validating the presence of name...")
			b.Navigate(persistentPlayerURL)
			Eventually("#invalid-name").Should(b.Exist())
			Ω("#invalid-email").Should(b.Exist())
			Ω("button.submit").ShouldNot(b.BeEnabled())

			Ω("#name").Should(b.SetValue("John Player"))
			Eventually("#invalid-name").ShouldNot(b.Exist())

			By("validating a valid e-mail is present...")
			Ω("#email").Should(b.SetValue("john"))
			Consistently("#invalid-email").Should(b.Exist())

			Ω("#email").Should(b.SetValue("john@example.com"))
			Eventually("#invalid-email").ShouldNot(b.Exist())
			Eventually("button.submit").Should(b.BeEnabled())

			b.Click("#A")
			b.Click("#E")
			b.Click("#M")

			Eventually("#A.selected").Should(b.Exist())
			Eventually("#E.selected").Should(b.Exist())
			Eventually("#M.selected").Should(b.Exist())
			Ω(".selected").Should(b.HaveCount(3))

			Ω("#comments").Should(b.SetValue("Might be late on Tuesday"))
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player <john@example.com>: A,E,M"))
			Ω(le()).Should(BeFrom(conf.LunchtimeDiscoEmail))
			Ω(le()).Should(BeSentTo(conf.BossEmail))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player <john@example.com>: A,E,M")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: Might be late on Tuesday")))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 1")))
			Ω(le()).Should(HaveText(ContainSubstring("+ B - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ E - 1")))
			Ω(le()).Should(HaveText(ContainSubstring("+ M - 1")))

			By("...and we do see the player's games stored off")
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 1))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player <john@example.com>",
				GameKeys: []string{"A", "E", "M"},
				Comments: "Might be late on Tuesday",
			}))

			By("when the player comes back they see their games and commands")
			b.Navigate(indexURL)
			Eventually("#content.index").Should(b.Exist())
			b.Navigate(persistentPlayerURL)
			Eventually("#name").Should(b.HaveValue("John Player"))
			Ω("#email").Should(b.HaveValue("john@example.com"))
			Eventually("#A.selected").Should(b.Exist())
			Eventually("#E.selected").Should(b.Exist())
			Eventually("#M.selected").Should(b.Exist())
			Ω(".selected").Should(b.HaveCount(3))
			Ω("#comments").Should(b.HaveValue("Might be late on Tuesday"))

			By("the user can adjust their selection (and even change their name)")
			Ω("#name").Should(b.SetValue("John Player Jr."))
			Ω("#A").Should(b.Click())
			Ω("#B").Should(b.Click())
			Ω("#comments").Should(b.SetValue("I'm not late anymore"))
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: E,M,B"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. <john@example.com>: E,M,B")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm not late anymore")))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ B - 1")))
			Ω(le()).Should(HaveText(ContainSubstring("+ E - 1")))
			Ω(le()).Should(HaveText(ContainSubstring("+ M - 1")))

			By("...and we do see the player's updates stored off")
			Eventually(disco.GetSnapshot).Should(HaveGameCount("B", 1))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player Jr. <john@example.com>",
				GameKeys: []string{"E", "M", "B"},
				Comments: "I'm not late anymore",
			}))

			By("when the user clears out their games (but there is a comment) - we keep them around")
			b.Navigate(persistentPlayerURL)
			Eventually("#B").Should(b.Click())
			Ω("#E").Should(b.Click())
			Ω("#M").Should(b.Click())
			Eventually(".selected").Should(b.HaveCount(0))
			Ω("#comments").Should(b.SetValue("I'm out, sorry"))
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: No Games"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. <john@example.com>: No Games")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm out, sorry")))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ B - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ E - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ M - 0")))
			Eventually(disco.GetSnapshot).Should(HaveGameCount("B", 0))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player Jr. <john@example.com>",
				GameKeys: []string{},
				Comments: "I'm out, sorry",
			}))

			outbox.Clear()
			By("when the user clears out their comment too")
			b.Navigate(persistentPlayerURL)
			Eventually("#comments").Should(b.SetValue(""))
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: No Games"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. <john@example.com>: No Games")))
			Ω(le()).ShouldNot(HaveText(ContainSubstring("Comment")))
			Ω(le()).Should(HaveText(ContainSubstring("+ A - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ B - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ E - 0")))
			Ω(le()).Should(HaveText(ContainSubstring("+ M - 0")))

			By("and the user is no longer in the set of participants")
			Ω(disco.GetSnapshot().Participants).Should(BeEmpty())

			By("but note that we keep them around as a historical participant")
			Ω(disco.HistoricalParticipants).Should(ConsistOf(mail.EmailAddress("John Player Jr. <john@example.com>")))

			By("and when the user comes back they see that they are empty but can sign up again")
			b.Navigate(persistentPlayerURL)
			Eventually("#name").Should(b.HaveValue("John Player Jr."))
			Ω("#email").Should(b.HaveValue("john@example.com"))
			b.Click("#A")
			Eventually(".selected").Should(b.HaveCount(1))
			b.Click("button.submit")

			By("the boss gets an acknowledgement email and the player is back!")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: A"))
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 1))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player Jr. <john@example.com>",
				GameKeys: []string{"A"},
				Comments: "",
			}))
			Ω(disco.HistoricalParticipants).Should(ConsistOf(mail.EmailAddress("John Player Jr. <john@example.com>")))

			By("finally, we validate that we've been backing things up along the way")
			rawSnapshot, err := db.FetchObject(KEY)
			Ω(err).ShouldNot(HaveOccurred())
			var backupSnapshot lunchtimedisco.LunchtimeDiscoSnapshot
			Ω(json.Unmarshal(rawSnapshot, &backupSnapshot)).Should(Succeed())
			Ω(backupSnapshot.Participants).Should(Equal(disco.GetSnapshot().Participants))

			rawHistoricalParticipants, err := db.FetchObject(PARTICIPANTS_KEY)
			Ω(err).ShouldNot(HaveOccurred())
			var backupHistoricalParticipants []mail.EmailAddress
			Ω(json.Unmarshal(rawHistoricalParticipants, &backupHistoricalParticipants)).Should(Succeed())
			Ω(backupHistoricalParticipants).Should(ConsistOf(mail.EmailAddress("John Player Jr. <john@example.com>")))
		})
	})

	Describe("allowing the boss to see who has signed up (via email)", func() {
		BeforeEach(func() {
			sendInvite()
			signUpPlayer(playerName, playerEmail.Address(), "I'm in", []string{"A", "B", "C"})
			signUpPlayer("Bob Player", "bob@example.com", "", []string{"A", "E"})
			signUpPlayer("Sally Player", "sally@example.com", "Let's play!", []string{"B", "E"})
			Eventually(le).Should(HaveSubject("Set Games - Sally Player <sally@example.com>: B,E"))
			outbox.Clear()
			clock.Fire()
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
		})

		It("includes the set of players in the monitor email", func() {
			Ω(le()).Should(HaveText(ContainSubstring(`+ A - 2 - Tuesday 9/26 at 10:00am - %s
  - John Player <player@example.com>
  - Bob Player <bob@example.com>
+ B - 2 - Tuesday 9/26 at 11:00am - %s
  - John Player <player@example.com>
  - Sally Player <sally@example.com>
+ C - 1 - Tuesday 9/26 at 12:00pm - %s
  - John Player <player@example.com>
+ D - 0 - Tuesday 9/26 at 1:00pm - %s
+ E - 2 - Wednesday 9/27 at 10:00am - %s
  - Bob Player <bob@example.com>
  - Sally Player <sally@example.com>`,
				forecast, forecast, forecast, forecast, forecast)))
		})
	})

	Describe("allowing the boss to see and modify who has signed up (via web)", func() {
		BeforeEach(func() {
			sendInvite()
			signUpPlayer(playerName, playerEmail.Address(), "I'm in", []string{"A", "B", "C"})
			signUpPlayer("Bob Player", "bob@example.com", "", []string{"A", "E"})
			signUpPlayer("Sally Player", "sally@example.com", "Let's play!", []string{"B", "E"})
			signUpPlayer("Flakey McFlake", "flakey@example.com", "I'm so in!", []string{"A", "B", "C"})
			signUpPlayer("Flakey McFlake", "flakey@example.com", "", []string{}) // now flakey is in historical participants, but not actually a player

			Eventually(le).Should(HaveSubject("Set Games - Flakey McFlake <flakey@example.com>: No Games"))
			outbox.Clear()
		})

		It("shows the boss the set of players", func() {
			b.Navigate(bossURL)
			Eventually(".pc").Should(b.HaveCount(3))
			Ω(b.InnerTextForEach(".pc-name")).Should(HaveExactElements("John Player", "Bob Player", "Sally Player"))
			Ω(b.InnerTextForEach(".pc-email")).Should(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com"))
			Ω(b.InnerTextForEach(".pc-comment")).Should(HaveExactElements("I'm in", "Let's play!"))

			Ω("#A").Should(b.HaveInnerText(HavePrefix("10AM\n2\nJohn Player\nBob Player\n🌤️")))
			Ω("#B").Should(b.HaveInnerText(HavePrefix("11AM\n2\nJohn Player\nSally Player\n🌤️")))
			Ω("#C").Should(b.HaveInnerText(HavePrefix("12PM\n1\nJohn Player\n🌤️")))
			Ω("#D").Should(b.HaveInnerText(HavePrefix("1PM\n0\n🌤️")))
			Ω("#E").Should(b.HaveInnerText(HavePrefix("10AM\n2\nBob Player\nSally Player\n🌤️")))

			Ω("#participant-address").Should(b.HaveValue(""))
			Ω(b.GetPropertyForEach("#historical-participants option", "value")).Should(HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flakey McFlake <flakey@example.com>",
			))

			Ω("td.game.selected").Should(b.HaveCount(0))
			b.Click(b.XPath("div").WithClass("pc-name").WithTextContains("John Player"))
			Eventually("#participant-address").Should(b.HaveValue("John Player <player@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(3))
			Ω("#A").Should(b.HaveClass("selected"))
			Ω("#B").Should(b.HaveClass("selected"))
			Ω("#C").Should(b.HaveClass("selected"))
			Ω("#comments").Should(b.HaveValue("I'm in"))

			b.Click(b.XPath("div").WithClass("pc-name").WithTextContains("Bob Player"))
			Eventually("#participant-address").Should(b.HaveValue("Bob Player <bob@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(2))
			Ω("#A").Should(b.HaveClass("selected"))
			Ω("#B").ShouldNot(b.HaveClass("selected"))
			Ω("#E").Should(b.HaveClass("selected"))
			Ω("#comments").Should(b.HaveValue(""))
		})

		It("allows the boss to modify sign ups", func() {
			b.Navigate(bossURL)
			Eventually(b.XPath("div").WithClass("pc-name").WithTextContains("John Player")).Should(b.Click())
			Eventually("#participant-address").Should(b.HaveValue("John Player <player@example.com>"))
			b.Click("#A")
			b.Click("#E")
			Ω("#comments").Should(b.SetValue("I'm more in"))
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - John Player <player@example.com>: B,C,E"))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm more in")))

			By("John will see the changes when they next log in!")
			b.Navigate(playerURL)
			Eventually("#name").Should(b.SetValue("John Player"))
			Ω("#email").Should(b.SetValue("player@example.com"))
			Eventually("#comments").Should(b.HaveValue("I'm more in"))
			Ω("#A").ShouldNot(b.HaveClass("selected"))
			Ω("#B").Should(b.HaveClass("selected"))
			Ω("#C").Should(b.HaveClass("selected"))
			Ω("#E").Should(b.HaveClass("selected"))
		})

		It("allows the boss to sign up a historical participant", func() {
			b.Navigate(bossURL)
			Eventually("#participant-address").Should(b.SetValue("Flaker <flakey@example.com>")) //note that we are testing the name updates too
			Eventually("button.submit").Should(b.BeEnabled())
			b.Click("#A")
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - Flaker <flakey@example.com>: A"))

			b.Navigate(bossURL)
			Eventually(".pc").Should(b.HaveCount(4))
			Ω(b.InnerTextForEach(".pc-name")).Should(HaveExactElements("John Player", "Bob Player", "Sally Player", "Flaker"))
			Ω(b.InnerTextForEach(".pc-email")).Should(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com", "flakey@example.com"))
			Ω(b.InnerTextForEach(".pc-comment")).Should(HaveExactElements("I'm in", "Let's play!"))

			Ω("#A").Should(b.HaveInnerText(HavePrefix("10AM\n3\nJohn Player\nBob Player\nFlaker\n🌤️")))

			b.Click(b.XPath("div").WithClass("pc-name").WithTextContains("Flaker"))
			Eventually("#participant-address").Should(b.HaveValue("Flaker <flakey@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(1))
			Ω("#A").Should(b.HaveClass("selected"))

			Ω(b.GetPropertyForEach("#historical-participants option", "value")).Should(HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flaker <flakey@example.com>",
			))
		})

		It("allows the boss to create a new participant", func() {
			b.Navigate(bossURL)
			Eventually("#participant-address").Should(b.SetValue("Onsi Fakhouri <onsijoe@gmail.com>")) //note that we are testing the name updates too
			Eventually("button.submit").Should(b.BeEnabled())
			b.Click("#A")
			b.Click("#G")
			b.SetValue("#comments", "Yay!")
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - Onsi Fakhouri <onsijoe@gmail.com>: A,G"))

			b.Navigate(bossURL)
			Eventually(".pc").Should(b.HaveCount(4))
			Ω(b.InnerTextForEach(".pc-name")).Should(HaveExactElements("John Player", "Bob Player", "Sally Player", "Onsi Fakhouri"))
			Ω(b.InnerTextForEach(".pc-email")).Should(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com", "onsijoe@gmail.com"))
			Ω(b.InnerTextForEach(".pc-comment")).Should(HaveExactElements("I'm in", "Let's play!", "Yay!"))

			Ω("#A").Should(b.HaveInnerText(HavePrefix("10AM\n3\nJohn Player\nBob Player\nOnsi Fakhouri\n🌤️")))

			b.Click(b.XPath("div").WithClass("pc-name").WithTextContains("Onsi Fakhouri"))
			Eventually("#participant-address").Should(b.HaveValue("Onsi Fakhouri <onsijoe@gmail.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(2))
			Ω("#A").Should(b.HaveClass("selected"))
			Ω("#G").Should(b.HaveClass("selected"))

			Ω(b.GetPropertyForEach("#historical-participants option", "value")).Should(HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flakey McFlake <flakey@example.com>",
				"Onsi Fakhouri <onsijoe@gmail.com>",
			))
		})
	})

	Describe("boss sending invite", func() {
		It("sends an invite to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			Eventually("#invite").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Lunchtime Bible Park Frisbee - Week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`<a href="https://www.sedenverultimate.net/lunchtime/%s" target="_blank">Here are the options for this week</a>`, s.GUID)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			Eventually("#invite").Should(b.Click())
			Eventually("#additional-content").Should(b.SetValue("Lets do it **again**."))
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Lunchtime Bible Park Frisbee - Week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`Lets do it <strong>again</strong>.`)))
			Ω(le()).Should(HaveHTML(ContainSubstring(`<a href="https://www.sedenverultimate.net/lunchtime/%s" target="_blank">Here are the options for this week</a>`, s.GUID)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("keeps the home-page as-is", func() {
			sendInvite()
			b.Navigate(indexURL)
			Eventually(".status.lunchtime").Should(b.HaveClass("pending"))
		})
	})

	Describe("boss sending no-invite", func() {
		It("sends a no-invite to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			Eventually("#no-invite").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("No Lunchtime Bible Park Frisbee This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`No lunchtime game this week.`)))

			Ω(s).Should(HaveState(StateNoInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			Eventually("#no-invite").Should(b.Click())
			Eventually("#additional-content").Should(b.SetValue("Merry **Christmas**!"))
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("No Lunchtime Bible Park Frisbee This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`Merry <strong>Christmas</strong>!`)))
			Ω(le()).Should(HaveHTML(ContainSubstring(`No lunchtime game this week.`)))

			Ω(s).Should(HaveState(StateNoInviteSent))
		})

		It("sets the home-page to no-game", func() {
			b.Navigate(bossURL)
			Eventually("#no-invite").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Invite")).Should(b.Click())
			Eventually(le).Should(HaveSubject("No Lunchtime Bible Park Frisbee This Week"))

			b.Navigate(indexURL)
			Eventually(".status.lunchtime").Should(b.HaveClass("game-off"))
		})
	})

	Describe("boss sending a badger", func() {
		BeforeEach(func() {
			sendInvite()
		})

		It("sends a badger to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			Eventually("#badger").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Badger")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Need more players for Lunchtime game - week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`still looking for players</strong>.  Can anyone else join?`)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			Eventually("#badger").Should(b.Click())
			Eventually("#additional-content").Should(b.SetValue("Need **3** more."))
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Badger")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Need more players for Lunchtime game - week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`Need <strong>3</strong> more.`)))
			Ω(le()).ShouldNot(HaveHTML(ContainSubstring(`still looking for players`)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("keeps the home-page as-is", func() {
			b.Navigate(bossURL)
			Eventually("#badger").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send Badger")).Should(b.Click())
			Eventually(le).Should(HaveSubject("Need more players for Lunchtime game - week of " + weekOf))

			b.Navigate(indexURL)
			Eventually(".status.lunchtime").Should(b.HaveClass("pending"))
		})
	})

	Describe("boss calling game", func() {
		BeforeEach(func() {
			sendInvite()
			signUpPlayer(playerName, playerEmail.Address(), "I'm in", []string{"A", "B", "C"})
			signUpPlayer("Bob Player", "bob@example.com", "", []string{"A", "B"})
			signUpPlayer("Sally Player", "sally@example.com", "", []string{"B", "C"})

			b.Navigate(bossURL)
			Eventually("#game-on").Should(b.Click())
			Eventually(".game-option").Should(b.HaveCount(4 * 4))
		})

		It("doesn't allow the boss ot send the e-mail until they pick a game", func() {
			Consistently(b.XPath("button").WithClass("confirm-message").WithText("Send Game On")).ShouldNot(b.BeEnabled())
		})

		It("shows the boss options", func() {
			Ω("#game-option-A .day").Should(b.HaveInnerText("Tue"))
			Ω("#game-option-A .time").Should(b.HaveInnerText("10AM"))
			Ω("#game-option-A .count").Should(b.HaveInnerText("2"))

			Ω("#game-option-B .day").Should(b.HaveInnerText("Tue"))
			Ω("#game-option-B .time").Should(b.HaveInnerText("11AM"))
			Ω("#game-option-B .count").Should(b.HaveInnerText("3"))

			Ω("#game-option-C .day").Should(b.HaveInnerText("Tue"))
			Ω("#game-option-C .time").Should(b.HaveInnerText("12PM"))
			Ω("#game-option-C .count").Should(b.HaveInnerText("2"))
		})

		Context("with the standard time", func() {
			BeforeEach(func() {
				b.Click("#game-option-B")
				Eventually("#game-option-B").Should(b.HaveClass("selected"))
				Ω("#additional-content").Should(b.SetValue("Yum **YUM**"))
				b.Click(b.XPath("button").WithClass("confirm-message").WithText("Send Game On"))
				Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
			})

			It("sends the game on email and updates the homepage", func() {
				Eventually(le).Should(HaveSubject("GAME ON! Tuesday 9/26 at 11:00am"))
				Ω(le()).Should(BeFrom(conf.BossEmail))
				Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

				Ω(le()).Should(HaveHTML(ContainSubstring(`Yum <strong>YUM</strong>`)))
				Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>GAME ON</strong> for <strong>Tuesday 9/26 at 11:00am</strong>`)))
				Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>Who</strong>: John, Bob and Sally`)))
			})

			It("sends a reminder e-mail on the morning-of", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
				Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! Tuesday 9/26 at 11:00am"))
				Ω(le()).Should(BeFrom(conf.BossEmail))
				Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

				Ω(le()).Should(HaveHTML(ContainSubstring(`Quick reminder: we`)))
			})

			It("updates the homepage", func() {
				b.Navigate(indexURL)
				Eventually(".status.lunchtime").Should(b.HaveClass("game-on"))
				Ω(".status.lunchtime .call .game-day").Should(b.HaveInnerText("Tuesday 9/26"))
				Ω(".status.lunchtime .call .game-time").Should(b.HaveInnerText("11AM"))
				Ω(".status.lunchtime .count .number").Should(b.HaveInnerText("3"))
				Ω(".status.lunchtime .count .text").Should(b.HaveInnerText("Players"))
			})
		})

		Context("with a time override", func() {
			BeforeEach(func() {
				b.Click("#game-option-B")
				Eventually("#game-option-B").Should(b.HaveClass("selected"))
				Ω("#override-start-time").Should(b.SetValue("11:15AM"))
				b.Click(b.XPath("button").WithClass("confirm-message").WithText("Send Game On"))
				Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
			})

			It("sends the game on email and updates the homepage", func() {
				Eventually(le).Should(HaveSubject("GAME ON! Tuesday 9/26 at 11:15AM"))
				Ω(le()).Should(BeFrom(conf.BossEmail))
				Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))
				Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>GAME ON</strong> for <strong>Tuesday 9/26 at 11:15AM</strong>`)))
				Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>Who</strong>: John, Bob and Sally`)))
			})

			It("sends a reminder e-mail on the morning-of", func() {
				clock.Fire()
				Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
				Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! Tuesday 9/26 at 11:15AM"))
				Ω(le()).Should(BeFrom(conf.BossEmail))
				Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

				Ω(le()).Should(HaveHTML(ContainSubstring(`Quick reminder: we`)))
			})

			It("updates the homepage", func() {
				b.Navigate(indexURL)
				Eventually(".status.lunchtime").Should(b.HaveClass("game-on"))
				Ω(".status.lunchtime .call .game-day").Should(b.HaveInnerText("Tuesday 9/26"))
				Ω(".status.lunchtime .call .game-time").Should(b.HaveInnerText("11:15AM"))
				Ω(".status.lunchtime .count .number").Should(b.HaveInnerText("3"))
				Ω(".status.lunchtime .count .text").Should(b.HaveInnerText("Players"))
			})
		})
	})

	Describe("boss calling no game", func() {
		BeforeEach(func() {
			sendInvite()
		})

		It("sends a no-game email to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			Eventually("#no-game").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Game")).Should(b.Click())
			Eventually(le).Should(HaveSubject("No Lunchtime Game This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>No lunchtime game this week</strong>.`)))

			Ω(s).Should(HaveState(StateNoGameSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			Eventually("#no-game").Should(b.Click())
			Eventually("#additional-content").Should(b.SetValue("Calling _it_."))
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Game")).Should(b.Click())
			Eventually(le).Should(HaveSubject("No Lunchtime Game This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>No lunchtime game this week</strong>.`)))
			Ω(le()).Should(HaveHTML(ContainSubstring(`Calling <em>it</em>.`)))

			Ω(s).Should(HaveState(StateNoGameSent))
		})

		It("sets the home-page to no-game", func() {
			b.Navigate(bossURL)
			Eventually("#no-game").Should(b.Click())
			Eventually(b.XPath("button").WithClass("confirm-message").WithText("Send No Game")).Should(b.Click())
			Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))

			b.Navigate(indexURL)
			Eventually(".status.lunchtime").Should(b.HaveClass("game-off"))
		})
	})
})
