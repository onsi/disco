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

	confirmSend := func(label string) {
		GinkgoHelper()
		b.Click(b.XPath("button").WithClass("confirm-message").WithText("Send " + label))
	}

	clickParticipant := func(name string) {
		GinkgoHelper()
		b.Click(b.ByCSS(".pc-name").ContainingText(name))
	}

	signUpPlayer := func(name string, email string, comments string, gameKeys []string) {
		GinkgoHelper()
		b.Navigate(playerURL)
		b.SetValue("#name", name)
		b.SetValue("#email", email)
		Eventually(".validation-error").ShouldNot(b.Exist())
		b.SetValue("#comments", comments)
		for _, key := range b.CurrentPropertyForEach(".selected", "id") {
			b.Click("#" + key.(string))
		}
		for _, key := range gameKeys {
			b.Click("#" + key)
			Eventually("#" + key).Should(b.HaveClass("selected"))
		}
		b.Click(".submit")
		Eventually(".message.set-games.success").Should(b.Exist())
	}

	sendInvite := func() {
		GinkgoHelper()
		//first we get the monitor e-mail
		clock.Fire()
		Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
		outbox.Clear()

		//then we send the invite
		b.Navigate(bossURL)
		b.Click("#invite")
		confirmSend("Invite")
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
			outbox.Clear()

			signUpPlayer(playerName, playerEmail.Address(), "", []string{"A"})
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 1))

			clock.Fire()
			Ω(clock.Time()).Should(BeOn(time.Monday, 6))
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
			outbox.Clear()

			signUpPlayer("Bob Player", "bob@example.com", "", []string{"A", "B"})
			Eventually(disco.GetSnapshot).Should(HaveGameCount("A", 2))

			clock.Fire()
			Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
			Eventually(le).Should(HaveSubject("Lunchtime Monitor: " + weekOf))
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
				b.Click("#no-invite")
				confirmSend("No Invite")
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
				b.Click("#no-game")
				confirmSend("No Game")
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
				b.Click("#game-on")
				b.Click("#game-option-E")
				confirmSend("Game On")
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
			b.Click("#invite")
			confirmSend("Invite")
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
			b.Click("#game-on")
			b.Click("#game-option-E")
			confirmSend("Game On")
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
			Eventually("#invalid-email").Should(b.Exist())
			Eventually("button.submit").ShouldNot(b.BeEnabled())

			b.SetValue("#name", "John Player")
			Eventually("#invalid-name").ShouldNot(b.Exist())

			By("validating a valid e-mail is present...")
			b.SetValue("#email", "john")
			Consistently("#invalid-email").Should(b.Exist())

			b.SetValue("#email", "john@example.com")
			Eventually("#invalid-email").ShouldNot(b.Exist())
			Eventually("button.submit").Should(b.BeEnabled())

			b.Click("#A")
			b.Click("#E")
			b.Click("#M")

			Eventually("#A.selected").Should(b.Exist())
			Eventually("#E.selected").Should(b.Exist())
			Eventually("#M.selected").Should(b.Exist())
			Eventually(".selected").Should(b.HaveCount(3))

			b.SetValue("#comments", "Might be late on Tuesday")
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player <john@example.com>: A,E,M"))
			Ω(le()).Should(BeFrom(conf.LunchtimeDiscoEmail))
			Ω(le()).Should(BeSentTo(conf.BossEmail))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player : A,E,M")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: Might be late on Tuesday")))

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
			Eventually("#email").Should(b.HaveValue("john@example.com"))
			Eventually("#A.selected").Should(b.Exist())
			Eventually("#E.selected").Should(b.Exist())
			Eventually("#M.selected").Should(b.Exist())
			Eventually(".selected").Should(b.HaveCount(3))
			Eventually("#comments").Should(b.HaveValue("Might be late on Tuesday"))

			By("the user can adjust their selection (and even change their name)")
			b.SetValue("#name", "John Player Jr.")
			b.Click("#A")
			b.Click("#B")
			b.SetValue("#comments", "I'm not late anymore")
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: E,M,B"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. : E,M,B")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm not late anymore")))

			By("...and we do see the player's updates stored off")
			Eventually(disco.GetSnapshot).Should(HaveGameCount("B", 1))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player Jr. <john@example.com>",
				GameKeys: []string{"E", "M", "B"},
				Comments: "I'm not late anymore",
			}))

			By("when the user clears out their games (but there is a comment) - we keep them around")
			b.Navigate(persistentPlayerURL)
			b.Click("#B")
			b.Click("#E")
			b.Click("#M")
			Eventually(".selected").Should(b.HaveCount(0))
			b.SetValue("#comments", "I'm out, sorry")
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: No Games"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. : No Games")))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm out, sorry")))
			Eventually(disco.GetSnapshot).Should(HaveGameCount("B", 0))
			Ω(disco.GetSnapshot().Participants[0]).Should(Equal(lunchtimedisco.LunchtimeParticipant{
				Address:  "John Player Jr. <john@example.com>",
				GameKeys: []string{},
				Comments: "I'm out, sorry",
			}))

			outbox.Clear()
			By("when the user clears out their comment too")
			b.Navigate(persistentPlayerURL)
			b.SetValue("#comments", "")
			b.Click("button.submit")

			By("the boss gets an acknowledgement email")
			Eventually(le).Should(HaveSubject("Set Games - John Player Jr. <john@example.com>: No Games"))
			Ω(le()).Should(HaveText(ContainSubstring("I've set games for John Player Jr. : No Games")))
			Ω(le()).ShouldNot(HaveText(ContainSubstring("Comment")))
			By("and the user is no longer in the set of participants")
			Ω(disco.GetSnapshot().Participants).Should(BeEmpty())

			By("but note that we keep them around as a historical participant")
			Ω(disco.HistoricalParticipants).Should(ConsistOf(mail.EmailAddress("John Player Jr. <john@example.com>")))

			By("and when the user comes back they see that they are empty but can sign up again")
			b.Navigate(persistentPlayerURL)
			Eventually("#name").Should(b.HaveValue("John Player Jr."))
			Eventually("#email").Should(b.HaveValue("john@example.com"))
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
			Eventually(".pc-name").Should(b.EachHaveInnerText(HaveExactElements("John Player", "Bob Player", "Sally Player")))
			Eventually(".pc-email").Should(b.EachHaveInnerText(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com")))
			Eventually(".pc-comment").Should(b.EachHaveInnerText(HaveExactElements("I'm in", "Let's play!")))

			Eventually("#A").Should(b.HaveInnerText(HavePrefix("10AM\n2\nJohn Player\nBob Player\n🌤️")))
			Eventually("#B").Should(b.HaveInnerText(HavePrefix("11AM\n2\nJohn Player\nSally Player\n🌤️")))
			Eventually("#C").Should(b.HaveInnerText(HavePrefix("12PM\n1\nJohn Player\n🌤️")))
			Eventually("#D").Should(b.HaveInnerText(HavePrefix("1PM\n0\n🌤️")))
			Eventually("#E").Should(b.HaveInnerText(HavePrefix("10AM\n2\nBob Player\nSally Player\n🌤️")))

			Eventually("#participant-address").Should(b.HaveValue(""))
			Eventually("#historical-participants option").Should(b.EachHaveProperty("value", HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flakey McFlake <flakey@example.com>",
			)))

			Eventually("td.game.selected").Should(b.HaveCount(0))
			clickParticipant("John Player")
			Eventually("#participant-address").Should(b.HaveValue("John Player <player@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(3))
			Eventually("#A").Should(b.HaveClass("selected"))
			Eventually("#B").Should(b.HaveClass("selected"))
			Eventually("#C").Should(b.HaveClass("selected"))
			Eventually("#comments").Should(b.HaveValue("I'm in"))

			clickParticipant("Bob Player")
			Eventually("#participant-address").Should(b.HaveValue("Bob Player <bob@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(2))
			Eventually("#A").Should(b.HaveClass("selected"))
			Eventually("#B").ShouldNot(b.HaveClass("selected"))
			Eventually("#E").Should(b.HaveClass("selected"))
			Eventually("#comments").Should(b.HaveValue(""))
		})

		It("allows the boss to modify sign ups", func() {
			b.Navigate(bossURL)
			clickParticipant("John Player")
			Eventually("#participant-address").Should(b.HaveValue("John Player <player@example.com>"))
			b.Click("#A")
			b.Click("#E")
			b.SetValue("#comments", "I'm more in")
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - John Player <player@example.com>: B,C,E"))
			Ω(le()).Should(HaveText(ContainSubstring("Comment: I'm more in")))

			By("John will see the changes when they next log in!")
			b.Navigate(playerURL)
			b.SetValue("#name", "John Player")
			b.SetValue("#email", "player@example.com")
			Eventually("#comments").Should(b.HaveValue("I'm more in"))
			Eventually("#A").ShouldNot(b.HaveClass("selected"))
			Eventually("#B").Should(b.HaveClass("selected"))
			Eventually("#C").Should(b.HaveClass("selected"))
			Eventually("#E").Should(b.HaveClass("selected"))
		})

		It("allows the boss to sign up a historical participant", func() {
			b.Navigate(bossURL)
			b.SetValue("#participant-address", "Flaker <flakey@example.com>") //note that we are testing the name updates too
			Eventually("button.submit").Should(b.BeEnabled())
			b.Click("#A")
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - Flaker <flakey@example.com>: A"))

			b.Navigate(bossURL)
			Eventually(".pc").Should(b.HaveCount(4))
			Eventually(".pc-name").Should(b.EachHaveInnerText(HaveExactElements("John Player", "Bob Player", "Sally Player", "Flaker")))
			Eventually(".pc-email").Should(b.EachHaveInnerText(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com", "flakey@example.com")))
			Eventually(".pc-comment").Should(b.EachHaveInnerText(HaveExactElements("I'm in", "Let's play!")))

			Eventually("#A").Should(b.HaveInnerText(HavePrefix("10AM\n3\nJohn Player\nBob Player\nFlaker\n🌤️")))

			clickParticipant("Flaker")
			Eventually("#participant-address").Should(b.HaveValue("Flaker <flakey@example.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(1))
			Eventually("#A").Should(b.HaveClass("selected"))

			Eventually("#historical-participants option").Should(b.EachHaveProperty("value", HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flaker <flakey@example.com>",
			)))
		})

		It("allows the boss to create a new participant", func() {
			b.Navigate(bossURL)
			b.SetValue("#participant-address", "Onsi Fakhouri <onsijoe@gmail.com>") //note that we are testing the name updates too
			Eventually("button.submit").Should(b.BeEnabled())
			b.Click("#A")
			b.Click("#G")
			b.SetValue("#comments", "Yay!")
			b.Click("button.submit")

			Eventually(le).Should(HaveSubject("Set Games - Onsi Fakhouri <onsijoe@gmail.com>: A,G"))

			b.Navigate(bossURL)
			Eventually(".pc").Should(b.HaveCount(4))
			Eventually(".pc-name").Should(b.EachHaveInnerText(HaveExactElements("John Player", "Bob Player", "Sally Player", "Onsi Fakhouri")))
			Eventually(".pc-email").Should(b.EachHaveInnerText(HaveExactElements("player@example.com", "bob@example.com", "sally@example.com", "onsijoe@gmail.com")))
			Eventually(".pc-comment").Should(b.EachHaveInnerText(HaveExactElements("I'm in", "Let's play!", "Yay!")))

			Eventually("#A").Should(b.HaveInnerText(HavePrefix("10AM\n3\nJohn Player\nBob Player\nOnsi Fakhouri\n🌤️")))

			clickParticipant("Onsi Fakhouri")
			Eventually("#participant-address").Should(b.HaveValue("Onsi Fakhouri <onsijoe@gmail.com>"))
			Eventually("td.game.selected").Should(b.HaveCount(2))
			Eventually("#A").Should(b.HaveClass("selected"))
			Eventually("#G").Should(b.HaveClass("selected"))

			Eventually("#historical-participants option").Should(b.EachHaveProperty("value", HaveExactElements(
				"John Player <player@example.com>",
				"Bob Player <bob@example.com>",
				"Sally Player <sally@example.com>",
				"Flakey McFlake <flakey@example.com>",
				"Onsi Fakhouri <onsijoe@gmail.com>",
			)))
		})
	})

	Describe("boss sending invite", func() {
		It("sends an invite to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			b.Click("#invite")
			confirmSend("Invite")
			Eventually(le).Should(HaveSubject("Lunchtime Bible Park Frisbee - Week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`<a href="https://www.sedenverultimate.net/lunchtime/%s" target="_blank">Here are the options for this week</a>`, s.GUID)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			b.Click("#invite")
			b.SetValue("#additional-content", "Lets do it **again**.")
			confirmSend("Invite")
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
			b.Click("#no-invite")
			confirmSend("No Invite")
			Eventually(le).Should(HaveSubject("No Lunchtime Bible Park Frisbee This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`No lunchtime game this week.`)))

			Ω(s).Should(HaveState(StateNoInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			b.Click("#no-invite")
			b.SetValue("#additional-content", "Merry **Christmas**!")
			confirmSend("No Invite")
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
			b.Click("#no-invite")
			confirmSend("No Invite")
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
			b.Click("#badger")
			confirmSend("Badger")
			Eventually(le).Should(HaveSubject("Need more players for Lunchtime game - week of " + weekOf))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`still looking for players</strong>.  Can anyone else join?`)))

			Ω(s).Should(HaveState(StateInviteSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			b.Click("#badger")
			b.SetValue("#additional-content", "Need **3** more.")
			confirmSend("Badger")
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
			b.Click("#badger")
			confirmSend("Badger")
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
			b.Click("#game-on")
			Eventually(".game-option").Should(b.HaveCount(4 * 4))
		})

		It("doesn't allow the boss ot send the e-mail until they pick a game", func() {
			Consistently(b.XPath("button").WithClass("confirm-message").WithText("Send Game On")).ShouldNot(b.BeEnabled())
		})

		It("shows the boss options", func() {
			Eventually("#game-option-A .day").Should(b.HaveInnerText("Tue"))
			Eventually("#game-option-A .time").Should(b.HaveInnerText("10AM"))
			Eventually("#game-option-A .count").Should(b.HaveInnerText("2"))

			Eventually("#game-option-B .day").Should(b.HaveInnerText("Tue"))
			Eventually("#game-option-B .time").Should(b.HaveInnerText("11AM"))
			Eventually("#game-option-B .count").Should(b.HaveInnerText("3"))

			Eventually("#game-option-C .day").Should(b.HaveInnerText("Tue"))
			Eventually("#game-option-C .time").Should(b.HaveInnerText("12PM"))
			Eventually("#game-option-C .count").Should(b.HaveInnerText("2"))
		})

		Context("with the standard time", func() {
			BeforeEach(func() {
				b.Click("#game-option-B")
				Eventually("#game-option-B").Should(b.HaveClass("selected"))
				b.SetValue("#additional-content", "Yum **YUM**")
				confirmSend("Game On")
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
				Eventually(".status.lunchtime .call .game-day").Should(b.HaveInnerText("Tuesday 9/26"))
				Eventually(".status.lunchtime .call .game-time").Should(b.HaveInnerText("11AM"))
				Eventually(".status.lunchtime .count .number").Should(b.HaveInnerText("3"))
				Eventually(".status.lunchtime .count .text").Should(b.HaveInnerText("Players"))
			})
		})

		Context("with a time override", func() {
			BeforeEach(func() {
				b.Click("#game-option-B")
				Eventually("#game-option-B").Should(b.HaveClass("selected"))
				b.SetValue("#override-start-time", "11:15AM")
				confirmSend("Game On")
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
				Eventually(".status.lunchtime .call .game-day").Should(b.HaveInnerText("Tuesday 9/26"))
				Eventually(".status.lunchtime .call .game-time").Should(b.HaveInnerText("11:15AM"))
				Eventually(".status.lunchtime .count .number").Should(b.HaveInnerText("3"))
				Eventually(".status.lunchtime .count .text").Should(b.HaveInnerText("Players"))
			})
		})
	})

	Describe("boss calling no game", func() {
		BeforeEach(func() {
			sendInvite()
		})

		It("sends a no-game email to the mailing list on behalf of the boss", func() {
			b.Navigate(bossURL)
			b.Click("#no-game")
			confirmSend("No Game")
			Eventually(le).Should(HaveSubject("No Lunchtime Game This Week"))
			s := disco.GetSnapshot()
			Ω(le()).Should(BeFrom(conf.BossEmail))
			Ω(le()).Should(BeSentTo(conf.LunchtimeDiscoList, conf.BossEmail))

			Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>No lunchtime game this week</strong>.`)))

			Ω(s).Should(HaveState(StateNoGameSent))
		})

		It("can include an optional message", func() {
			b.Navigate(bossURL)
			b.Click("#no-game")
			b.SetValue("#additional-content", "Calling _it_.")
			confirmSend("No Game")
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
			b.Click("#no-game")
			confirmSend("No Game")
			Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))

			b.Navigate(indexURL)
			Eventually(".status.lunchtime").Should(b.HaveClass("game-off"))
		})
	})
})
