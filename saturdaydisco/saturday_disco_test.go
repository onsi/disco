package saturdaydisco_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	clockpkg "github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	. "github.com/onsi/disco/saturdaydisco"
	"github.com/onsi/disco/weather"
)

type saturdayDiscoTestConfig struct {
	Now         time.Time
	GameDate    string
	Description string
	Offset      int
}

var testConfigs = []saturdayDiscoTestConfig{
	{
		Now:         time.Date(2023, time.September, 24, 0, 0, 0, 0, clockpkg.Timezone), // a Sunday
		GameDate:    "9/30",                                                             //the following Saturday
		Description: "during DST",
		Offset:      0,
	},
	{
		Now:         time.Date(2023, time.November, 12, 0, 0, 0, 0, clockpkg.Timezone), // a Sunday
		GameDate:    "11/18",                                                           //the following Saturday
		Description: "when not DST",
		Offset:      30,
	},
}

var _ = Describe("SaturdayDisco", func() {
	for _, testConfig := range testConfigs {
		testConfig := testConfig
		Describe(testConfig.Description, func() {
			var outbox *mail.FakeOutbox
			var clock *clockpkg.FakeAlarmClock
			var interpreter *FakeInterpreter
			var forecaster *weather.FakeForecaster
			var disco *SaturdayDisco
			var db *s3db.FakeS3DB
			var conf config.Config

			var now time.Time
			var gameDate string
			var playerEmail mail.EmailAddress

			var le func() mail.Email
			var handleIncomingEmail = func(m mail.Email) mail.Email {
				GinkgoHelper()
				m.MessageID = uuid.New().String() //fake the message ID to simulate how real e-mails behave
				disco.HandleIncomingEmail(m)
				return m
			}
			var bossToDisco = func(args ...string) mail.Email {
				GinkgoHelper()
				body, subject := "", "hey"
				if len(args) == 1 {
					body = args[0]
				} else if len(args) == 2 {
					subject = args[0]
					body = args[1]
				} else {
					Expect(args).To(HaveLen(2), "bossToDisco takes either a body or a subject and a body")
				}
				return handleIncomingEmail(mail.E().
					WithFrom(conf.BossEmail).
					WithTo(conf.SaturdayDiscoEmail).
					WithSubject(subject).
					WithBody(body))
			}

			BeforeEach(func() {
				outbox = mail.NewFakeOutbox()
				le = outbox.LastEmail
				clock = clockpkg.NewFakeAlarmClock()
				interpreter = NewFakeInterpreter()
				forecaster = weather.NewFakeForecaster()
				forecaster.SetForecast(weather.Forecast{
					Temperature:                72,
					TemperatureUnit:            "F",
					WindSpeed:                  "8 mph",
					ProbabilityOfPrecipitation: 10,
					ShortForecast:              "Partly Cloud",
					ShortForecastEmoji:         "🌤️",
				})
				db = s3db.NewFakeS3DB()
				conf.BossEmail = mail.EmailAddress("Boss <boss@example.com>")
				conf.SaturdayDiscoEmail = mail.EmailAddress("Disco <saturday-disco@sedenverultimate.net>")
				conf.SaturdayDiscoList = mail.EmailAddress("Saturday-List <saturday-se-denver-ultimate@googlegroups.com>")
				playerEmail = mail.EmailAddress("player@example.com")

				now = testConfig.Now
				gameDate = testConfig.GameDate
				clock.SetTime(now)

				isStartup, _ := CurrentSpecReport().MatchesLabelFilter("startup")
				if !isStartup {
					var err error
					disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
					Ω(err).ShouldNot(HaveOccurred())
					DeferCleanup(disco.Stop)
					Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
					outbox.Clear() //clear out the welcome email
				}
			})

			Describe("startup and persistence", Label("startup"), func() {
				put := func(snapshot SaturdayDiscoSnapshot) {
					data, err := json.Marshal(snapshot)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(db.PutObject(KEY, data)).Should(Succeed())
				}
				fetch := func() SaturdayDiscoSnapshot {
					data, err := db.FetchObject(KEY)
					Ω(err).ShouldNot(HaveOccurred())
					var snapshot SaturdayDiscoSnapshot
					Ω(json.Unmarshal(data, &snapshot)).Should(Succeed())
					return snapshot
				}

				Describe("backing up regularly", func() {
					It("saves the backup whenever a command occurs", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).ShouldNot(HaveOccurred())
						bossToDisco("/set onsijoe@gmail.com 2")
						Eventually(disco.GetSnapshot).Should(HaveCount(2))
						Ω(fetch().Participants).Should(Equal(disco.GetSnapshot().Participants))
					})

					It("saves the backup whenever a schedule event occurs", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).ShouldNot(HaveOccurred())
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
						Ω(fetch().State).Should(Equal(disco.GetSnapshot().State))
					})
				})

				Context("when there is no backup stored in the database", func() {
					It("starts afresh and sends an email", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(le()).Should(HaveSubject("SaturdayDisco Joined the Dance Floor"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("I'm up and running now:\nNo backup found, starting from scratch...")))
						Ω(le()).Should(HaveText(ContainSubstring("Current State: pending")))
						Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						Ω(disco.GetSnapshot()).Should(HaveCount(0))

						outbox.Clear()
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Tuesday, 6, testConfig.Offset))
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
					})
				})

				Context("if the backup fails to load", func() {
					BeforeEach(func() {
						db.SetFetchError(fmt.Errorf("boom"))
					})

					It("returns an error and sends an email", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).Should(HaveOccurred())
						Ω(le()).Should(HaveSubject("SaturdayDisco FAILED to Join the Dance Floor"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("FAILED TO LOAD BACKUP: boom")))
					})
				})

				Context("if the backup fails to unmarshal", func() {
					BeforeEach(func() {
						Ω(db.PutObject(KEY, []byte("ß"))).Should(Succeed())
					})

					It("returns an error and sends an email", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).Should(HaveOccurred())
						Ω(le()).Should(HaveSubject("SaturdayDisco FAILED to Join the Dance Floor"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("FAILED TO UNMARSHAL BACKUP: %s", err.Error())))
					})
				})

				Context("if the backup is from a prior game week", func() {
					BeforeEach(func() {
						put(SaturdayDiscoSnapshot{
							State: StateRequestedGameOnApproval,
							Participants: Participants{
								Participant{Address: playerEmail, Count: 2},
							},
							T:         clockpkg.NextSaturdayAt10Or1030(now.Add(-time.Hour * 24 * 7)),
							NextEvent: clockpkg.NextSaturdayAt10Or1030(now.Add(-time.Hour * 24 * 7)).Add(-2*time.Hour*24 + 8*time.Hour),
						})
					})

					It("discards the backup, starts afresh, and sends an eamil", func() {
						var err error
						disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(le()).Should(HaveSubject("SaturdayDisco Joined the Dance Floor"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("I'm up and running now:\nBackup is from a previous week.  Resetting.")))
						Ω(le()).Should(HaveText(ContainSubstring("Current State: pending")))
						Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						Ω(disco.GetSnapshot()).Should(HaveCount(0))

						outbox.Clear()
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Tuesday, 6, testConfig.Offset))
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
					})
				})

				Context("if the backup is good", func() {
					BeforeEach(func() {
						put(SaturdayDiscoSnapshot{
							State: StateRequestedGameOnApproval,
							Participants: Participants{
								Participant{Address: playerEmail, Count: 2},
								Participant{Address: "onsijoe@gmail.com", Count: 6}, //have quorum
							},
							T:         clockpkg.NextSaturdayAt10Or1030(now),
							NextEvent: clockpkg.NextSaturdayAt10Or1030(now).Add(-2*time.Hour*24 + 4*time.Hour),
						})
						clock.SetTime(clockpkg.NextSaturdayAt10Or1030(now).Add(-2*time.Hour*24 + 3*time.Hour))
					})

					Context("if it's not time for NextEvent yet", func() {
						BeforeEach(func() {
							clock.SetTime(clockpkg.NextSaturdayAt10Or1030(now).Add(-2*time.Hour*24 + 3*time.Hour))
						})

						It("spins up and picks up where it left off (and sends an e-mail)", func() {
							var err error
							disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(le()).Should(HaveSubject("SaturdayDisco Joined the Dance Floor"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("I'm up and running now:\nBackup is good.  Spinning up...")))
							Ω(le()).Should(HaveText(ContainSubstring("Current State: requested_game_on_approval")))
							Ω(le()).Should(HaveText(ContainSubstring("onsijoe@gmail.com: 6")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
							Ω(disco.GetSnapshot()).Should(HaveCount(8))

							outbox.Clear()
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Thursday, 14, testConfig.Offset))
							Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
							Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
						})
					})

					Context("if it's already past time for the next event", func() {
						BeforeEach(func() {
							clock.SetTime(clockpkg.NextSaturdayAt10Or1030(now).Add(-2*time.Hour*24 + 3*time.Hour))
							go clock.Fire() //basically what happens irl
						})

						It("spins up and picks up where it left off (and sends an e-mail)", func() {
							var err error
							outbox.Clear()
							disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
							Ω(err).ShouldNot(HaveOccurred())
							Eventually(outbox.Emails).Should(HaveLen(2))

							startupEmail := outbox.Emails()[0]
							Ω(startupEmail).Should(HaveSubject("SaturdayDisco Joined the Dance Floor"))
							Ω(startupEmail).Should(BeSentTo(conf.BossEmail))
							Ω(startupEmail).Should(HaveText(ContainSubstring("I'm up and running now:\nBackup is good.  Spinning up...")))
							Ω(startupEmail).Should(HaveText(ContainSubstring("Current State: requested_game_on_approval")))
							Ω(startupEmail).Should(HaveText(ContainSubstring("onsijoe@gmail.com: 6")))

							gameOnEmail := outbox.Emails()[1]
							Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
							Ω(gameOnEmail).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
							Ω(gameOnEmail).Should(HaveText(ContainSubstring("onsijoe (6)")))
							Ω(disco.GetSnapshot()).Should(HaveCount(8))
						})
					})
				})

				for _, state := range []SaturdayDiscoState{StatePending, StateRequestedInviteApproval} {
					state := state
					Context("if the invite hasn't been sent yet ("+string(state)+") and its after thursday 2pm", func() {
						BeforeEach(func() {
							put(SaturdayDiscoSnapshot{
								State: state,
								Participants: Participants{
									Participant{Address: playerEmail, Count: 2},
									Participant{Address: "onsijoe@gmail.com", Count: 6}, //have quorum
								},
								T:         clockpkg.NextSaturdayAt10Or1030(now),
								NextEvent: clockpkg.NextSaturdayAt10Or1030(now).Add(-4*time.Hour*24 - 4*time.Hour),
							})
							clock.SetTime(clockpkg.NextSaturdayAt10Or1030(now).Add(-2*time.Hour*24 + 4*time.Hour))
						})

						It("aborts and sends an email", func() {
							var err error
							disco, err = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox, interpreter, forecaster, db)
							Ω(err).ShouldNot(HaveOccurred())
							Ω(le()).Should(HaveSubject("SaturdayDisco Joined the Dance Floor"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("I'm up and running now:\nBackup is good.  Spinning up...\nIt's after Thursday at 2pm and we haven't sent the invite yet.  Aborting.  You'll need to take over, boss.")))
							Ω(le()).Should(HaveText(ContainSubstring("Current State: abort")))
							Ω(le()).Should(HaveText(ContainSubstring("onsijoe@gmail.com: 6")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateAbort))
							Ω(disco.GetSnapshot()).Should(HaveCount(8))
						})
					})
				}
			})

			Describe("commands", func() {
				Describe("when an unknown command is sent by the boss", func() {
					It("replies with an error e-mail", func() {
						bossToDisco("/floop")
						Eventually(le).Should(HaveSubject("Re: hey"))
						Ω(le()).Should(HaveText(ContainSubstring("invalid command: /floop")))
					})
				})

				Describe("when the boss send an e-mail that includes the list", func() {
					It("totally ignores the boss' email, even if its a valid command", func() {
						handleIncomingEmail(mail.E().WithFrom(conf.BossEmail).WithTo(conf.SaturdayDiscoList, conf.SaturdayDiscoEmail).WithSubject("hey").WithBody("/set onsijoe@gmail.com 3"))
						Consistently(le).Should(BeZero())
						Ω(disco.GetSnapshot()).Should(HaveCount(0))
					})
				})

				Describe("when a player sends an e-mail that includes the list or boss", func() {
					Context("and disco is unsure if its a command", func() {
						It("does nothing", func() {
							interpreter.SetCommand(Command{CommandType: CommandPlayerUnsure})
							handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoList).WithSubject("hey").WithBody("I'm in!"))
							Consistently(le).Should(BeZero())
						})
					})

					Context("and disco is unsure if its a command", func() {
						It("does nothing", func() {
							interpreter.SetCommand(Command{CommandType: CommandPlayerUnsure})
							handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoEmail, conf.BossEmail).WithSubject("hey").WithBody("I'm in!"))
							Consistently(le).Should(BeZero())
						})
					})
				})

				Describe("when a player sends an e-mail that doesn't include the boss and disco is unsure", func() {
					It("replies and CCs the boss", func() {
						interpreter.SetCommand(Command{CommandType: CommandPlayerUnsure})
						handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoEmail, "someone-else@example.com").WithSubject("hey").WithBody("Make me a bagel."))
						Eventually(le).Should(HaveSubject("Re: hey"))
						Ω(le()).Should(BeSentTo(playerEmail, conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("I'm not sure what you're asking me to do.  I'm CCing the boss to help.")))
					})
				})

				Describe("when interpreting a player e-mail and an error occurs", func() {
					It("notifies the boss", func() {
						interpreter.SetError(fmt.Errorf("boom"))
						handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoList).WithSubject("hey").WithBody("I'm in!"))

						Eventually(le).Should(HaveSubject("Fwd: hey"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("I got an error while processing this email:\nboom")))
						Ω(le()).Should(HaveHTML(""))
					})
				})

				Describe("when an e-mail comes from disco itself", func() {
					It("completely ignores the e-mail", func() {
						interpreter.SetCommand(Command{CommandType: CommandPlayerStatus})
						handleIncomingEmail(mail.E().WithFrom(conf.SaturdayDiscoEmail).WithTo(conf.SaturdayDiscoList).WithSubject("hey").WithBody("Is the game on?"))
						Consistently(le).Should(BeZero())
					})
				})

				Describe("when an e-mail is received twice (a can happen when disco gets an e-mail directly *and* via the mailing list)", func() {
					It("only runs once", func() {
						outbox.Clear()
						message := bossToDisco("/set onsijoe@gmail.com 2")
						Eventually(disco.GetSnapshot).Should(HaveCount(2))
						bossToDisco("/set onsijoe@gmail.com 1") //some other message
						Eventually(disco.GetSnapshot).Should(HaveCount(1))

						disco.HandleIncomingEmail(message) //now the original message comes back
						Consistently(disco.GetSnapshot).Should(HaveCount(1))

						Ω(outbox.Emails()).Should(HaveLen(2)) // only two response e-mails were sent

						bossToDisco("/RESET-RESET-RESET") //when a reset occurs...
						Eventually(disco.GetSnapshot).Should(HaveCount(0))
						disco.HandleIncomingEmail(message) //...we clear the cache
						Eventually(disco.GetSnapshot).Should(HaveCount(2))

					})
				})

				Describe("registering players", func() {
					BeforeEach(func() {
						Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateInviteSent))
						outbox.Clear()
					})

					Describe("the admin interface for setting players", func() {
						It("allows the admin to add players", func() {
							bossToDisco("\n  /set   Onsi Fakhouri <onsijoe@gmail.com>   2   \n")
							Eventually(le).Should(HaveSubject("Re: hey"))
							Ω(le()).Should(HaveText(ContainSubstring("I've set Onsi Fakhouri <onsijoe@gmail.com> to 2")))
							Eventually(disco.GetSnapshot).Should(HaveCount(2))
							Ω(disco.GetSnapshot()).Should(HaveParticipantWithCount("onsijoe@gmail.com", 2))
						})

						It("allows the admin to set multiple players", func() {
							bossToDisco("/set Onsi Fakhouri <onsijoe@gmail.com> 2")
							Eventually(disco.GetSnapshot).Should(HaveCount(2))
							bossToDisco("/set player@example.com 4")
							Eventually(disco.GetSnapshot).Should(HaveCount(6))
							Ω(disco.GetSnapshot()).Should(HaveParticipantWithCount("onsijoe@gmail.com", 2))
							Ω(disco.GetSnapshot()).Should(HaveParticipantWithCount("player@example.com", 4))

							Ω(le()).Should(HaveSubject("Re: hey"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("I've set player@example.com to 4")))
							Ω(le()).Should(HaveText(ContainSubstring("Current State: invite_sent")))
							Ω(le()).Should(HaveText(ContainSubstring("Onsi Fakhouri <onsijoe@gmail.com>: 2")))
							Ω(le()).Should(HaveText(ContainSubstring("player@example.com: 4")))
							Ω(le()).Should(HaveText(ContainSubstring("Total Count: 6")))
							Ω(le()).Should(HaveText(ContainSubstring("Has Quorum: false")))
						})

						It("allows the admin to change a players' count", func() {
							bossToDisco("/set Onsi onsijoe@gmail.com 2")
							Eventually(disco.GetSnapshot).Should(HaveCount(2))
							bossToDisco("/set player@example.com 4")
							Eventually(disco.GetSnapshot).Should(HaveCount(6))
							Ω(le()).Should(HaveSubject("Re: hey"))
							Ω(le()).Should(HaveText(ContainSubstring("onsijoe@gmail.com: 2"))) // no name yet

							bossToDisco("/set Onsi Fakhouri <onsijoe@gmail.com> 6")
							Eventually(disco.GetSnapshot).Should(HaveCount(10))
							Ω(disco.GetSnapshot()).Should(HaveParticipantWithCount("onsijoe@gmail.com", 6))
							Ω(disco.GetSnapshot()).Should(HaveParticipantWithCount("player@example.com", 4))

							Ω(le()).Should(HaveSubject("Re: hey"))
							Ω(le()).Should(HaveText(ContainSubstring("Onsi Fakhouri <onsijoe@gmail.com>: 6"))) // we updated the name!
						})

						It("send back an error if the admin messes up", func() {
							bossToDisco("/set")
							Eventually(le).Should(HaveText(ContainSubstring("invalid command: /set")))
							bossToDisco("/set 2")
							Eventually(le).Should(HaveText(ContainSubstring("invalid command: /set 2")))
							bossToDisco("/set onsijoe@gmail.com two")
							Eventually(le).Should(HaveText(ContainSubstring("invalid command: /set onsijoe@gmail.com two")))
						})

						It("keeps track of all the emails associated with the player", func() {
							bossToDisco("/set Onsi Fakhouri <onsijoe@gmail.com> 2")
							Eventually(disco.GetSnapshot).Should(HaveCount(2))
							bossToDisco("/set player@example.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(3))
							bossToDisco("/set player@example.com 3")
							Eventually(disco.GetSnapshot).Should(HaveCount(5))
							bossToDisco("/set onsijoe@gmail.com 0")
							Eventually(disco.GetSnapshot).Should(HaveCount(3))

							bossToDisco("Status Please", "/status")
							Eventually(le).Should(HaveSubject("Re: Status Please"))
							buf := gbytes.BufferWithBytes([]byte(le().Text))
							Ω(buf).Should(gbytes.Say("Participants:"))
							Ω(buf).Should(gbytes.Say("- Onsi Fakhouri <onsijoe@gmail.com>: 0"))
							Ω(buf).Should(gbytes.Say("/set Onsi Fakhouri <onsijoe@gmail.com> 2"))
							Ω(buf).Should(gbytes.Say("/set onsijoe@gmail.com 0"))
							Ω(buf).Should(gbytes.Say("- player@example.com: 3"))
							Ω(buf).Should(gbytes.Say("/set player@example.com 1"))
							Ω(buf).Should(gbytes.Say("/set player@example.com 3"))
						})

						It("ignores the boss when he includes the list", func() {
							handleIncomingEmail(mail.E().
								WithFrom(conf.BossEmail).
								WithTo(conf.SaturdayDiscoList).
								WithSubject("hey").
								WithBody("/set onsijoe@gmail.com 2"))
							Consistently(le).Should(BeZero())
						})
					})

					Describe("the user-facing interface for setting players", func() {
						BeforeEach(func() {
							bossToDisco("/set player@example.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(1))
						})

						It("allows players to register themselves, forwarding the e-mail boss", func() {
							interpreter.SetCommand(Command{CommandType: CommandPlayerSetCount, Count: 2})
							handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoEmail, conf.SaturdayDiscoList, mail.EmailAddress("brother@example.com")).WithSubject("hey").WithBody("My brother's joining too!"))

							Eventually(disco.GetSnapshot).Should(HaveCount(2))
							Ω(le()).Should(HaveSubject("Fwd: hey"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("I've set the player's count to 2.")))
							Ω(le()).Should(HaveText(ContainSubstring("Send me a:\n\n/set player@example.com N")))
							Ω(le()).Should(HaveHTML(BeEmpty()))
						})
					})
				})

				Describe("getting status", func() {
					BeforeEach(func() {
						clock.Fire() // invite approval
						clock.Fire() // invite
						bossToDisco("/set random@example.com 0")
						bossToDisco("/set player@example.com 1")
						Eventually(disco.GetSnapshot).Should(HaveCount(1))
						bossToDisco("/set onsijoe@gmail.com 2")
						Eventually(disco.GetSnapshot).Should(HaveCount(3))
						outbox.Clear()
					})

					Describe("the boss' interface", func() {
						BeforeEach(func() {
							bossToDisco("/status")
							Eventually(le).Should(HaveSubject("Re: hey"))
						})

						It("returns status", func() {
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("Current State: invite_sent")))
							Ω(le()).Should(HaveText(ContainSubstring("Next Event on: %s", disco.GetSnapshot().NextEvent.Format("Monday 1/2 3:04pm"))))

							Ω(le()).Should(HaveText(ContainSubstring("Weather Forecast: 🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph")))
							Ω(le()).Should(HaveText(ContainSubstring("Participants:")))
							Ω(le()).Should(HaveText(ContainSubstring("- player@example.com: 1")))
							Ω(le()).Should(HaveText(ContainSubstring("- onsijoe@gmail.com: 2")))
							Ω(le()).Should(HaveText(ContainSubstring("Total Count: 3")))
							Ω(le()).Should(HaveText(ContainSubstring("Has Quorum: false")))

							Ω(le()).Should(HaveHTML(""))
						})
					})

					Describe("the player's interface", func() {
						sendPlayerRequest := func() {
							interpreter.SetCommand(Command{CommandType: CommandPlayerStatus})

							handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoEmail, conf.SaturdayDiscoList, mail.EmailAddress("random@example.com")).WithSubject("hey").WithBody("Is the game on?"))
							Eventually(le).Should(HaveSubject("Re: hey"))
						}

						Context("when the game hasn't been called yet", func() {
							BeforeEach(func() {
								sendPlayerRequest()
							})

							It("allows players to get a status update, replying to all", func() {
								Ω(le()).Should(BeSentTo(playerEmail, conf.BossEmail, conf.SaturdayDiscoList, mail.EmailAddress("random@example.com")))
								Ω(le()).Should(HaveText(ContainSubstring("The game on " + gameDate + " hasn't been called yet.")))
								Ω(le()).Should(HaveText(ContainSubstring("Weather Forecast: 🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph")))
								Ω(le()).Should(HaveText(ContainSubstring("Players: player and onsijoe (2)")))
								Ω(le()).Should(HaveText(ContainSubstring("Total: 3")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player and onsijoe <strong>(2)</strong>")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Total</strong>: 3")))
							})
						})

						Context("when the game is on", func() {
							BeforeEach(func() {
								bossToDisco("/game-on")
								Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
								sendPlayerRequest()
							})

							It("allows players to get a status update, replying to all", func() {
								Ω(le()).Should(BeSentTo(playerEmail, conf.BossEmail, conf.SaturdayDiscoList, mail.EmailAddress("random@example.com")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>GAME ON!</strong>")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Weather Forecast</strong>: 🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph")))
								Ω(le()).Should(HaveText(ContainSubstring("Players: player and onsijoe (2)")))
								Ω(le()).Should(HaveText(ContainSubstring("Total: 3")))
							})
						})

						Context("when the game is off", func() {
							BeforeEach(func() {
								bossToDisco("/no-game")
								Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
								sendPlayerRequest()
							})

							It("allows players to get a status update, replying to all", func() {
								Ω(le()).Should(BeSentTo(playerEmail, conf.BossEmail, conf.SaturdayDiscoList, mail.EmailAddress("random@example.com")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>NO GAME</strong>")))
								Ω(le()).Should(HaveText(ContainSubstring("Weather Forecast: 🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph")))
								Ω(le()).Should(HaveText(ContainSubstring("Players: player and onsijoe (2)")))
								Ω(le()).Should(HaveText(ContainSubstring("Total: 3")))
							})
						})

					})
				})

				Describe("aborting the scheduler", func() {
					BeforeEach(func() {
						bossToDisco("/abort")
						Eventually(disco.GetSnapshot).Should(HaveState(StateAbort))
					})

					It("it acks the boss and jumps to StateAbort, the next clock tick simply resets the system", func() {
						Ω(le()).Should(HaveSubject("Re: hey"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Alright.  I'm aborting.  You're on the hook for keeping eyes on things.")))
						Ω(le()).Should(HaveText(ContainSubstring("Current State: pending")))
						outbox.Clear()
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
						Ω(le()).Should(BeZero())
					})
				})

				Describe("resetting the system", func() {
					BeforeEach(func() {
						clock.Fire() // invite approval
						clock.Fire() // invite
						Eventually(disco.GetSnapshot).Should(HaveState(StateInviteSent))
						bossToDisco("/set onsijoe@gmail.com 3")
						Eventually(disco.GetSnapshot).Should(HaveCount(3))
					})

					It("resets the system", func() {
						bossToDisco("/RESET-RESET-RESET")
						Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
						Ω(disco.GetSnapshot()).Should(HaveCount(0))

						Ω(le()).Should(HaveSubject("Re: hey"))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Alright.  I'm resetting.  You'd better know what you're doing!")))
					})
				})

				Describe("if a player wants to unsubscribe", func() {
					BeforeEach(func() {
						interpreter.SetCommand(Command{CommandType: CommandPlayerUnsubscribe})
						handleIncomingEmail(mail.E().WithFrom(playerEmail).WithTo(conf.SaturdayDiscoEmail, conf.SaturdayDiscoList, mail.EmailAddress("random@example.com")).WithSubject("hey").WithBody("Please unsubscibe me."))
					})

					It("replies and includes boss", func() {
						Eventually(le).Should(HaveSubject("Re: hey"))
						Ω(le()).Should(BeSentTo(playerEmail, conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("I got your unsubscribe request.  I'm notifying the boss to remove you from the list.")))
					})
				})

				Describe("forcibly calling game on", func() {
					Context("with no additional content", func() {
						BeforeEach(func() {
							bossToDisco("/game-on")
							Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
						})

						It("sends the game-on email right away, no questions asked", func() {
							Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("Players: No one's signed up yet")))
						})
					})

					Context("with additional content", func() {
						BeforeEach(func() {
							bossToDisco("/game-on\n\nLETS **GO**")
							Eventually(disco.GetSnapshot).Should(HaveState(StateGameOnSent))
						})

						It("sends the game-on email right away, no questions asked and includes the additional content", func() {
							Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("Players: No one's signed up yet")))
							Ω(le()).Should(HaveHTML(ContainSubstring("LETS <strong>GO</strong>")))
						})
					})
				})

				Describe("forcibly calling game on", func() {
					Context("with no additional content", func() {
						BeforeEach(func() {
							bossToDisco("/no-game")
							Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
						})

						It("sends the no-game email right away, no questions asked", func() {
							Ω(le()).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("No Saturday game this week.")))
						})
					})

					Context("with additional content", func() {
						BeforeEach(func() {
							bossToDisco("/no-game\n\nTOO MUCH **SNOW**")
							Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
						})

						It("sends the no-game email right away, no questions asked and includes the additional content", func() {
							Ω(le()).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("No Saturday game this week.")))
							Ω(le()).Should(HaveHTML(ContainSubstring("TOO MUCH <strong>SNOW</strong>")))
						})
					})
				})
			})

			Describe("the flow of the state machine/scheduler", func() {
				Describe("sending invitations", func() {
					var approvalRequest mail.Email
					BeforeEach(func() {
						clock.Fire()
						Eventually(le).ShouldNot(BeZero())

						approvalRequest = le()
					})

					It("asks for permission to send an invitation on Tuesday at 6am", func() {
						Ω(clock.Time()).Should(BeOn(time.Tuesday, 6, testConfig.Offset))
						Ω(le()).Should(HaveSubject("[invite-approval-request] Can I send this week's invite?"))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
						Ω(le()).Should(HaveText(ContainSubstring("Ignore this e-mail to have me send the invite eventually (on %s)", clock.Time().Add(4*time.Hour).Format("Monday 1/2 3:04pm"))))
						Ω(le()).Should(HaveText(ContainSubstring("--- Invite Email ---\nSubject: Saturday Bible Park Frisbee " + gameDate)))
						Ω(le()).Should(HaveText(ContainSubstring("--- No Invite Email ---\nSubject: No Saturday Bible Park Frisbee This Week")))
						Ω(le()).Should(HaveHTML(BeEmpty()))
						Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedInviteApproval))
					})

					Context("if the boss doesn't reply", func() {
						BeforeEach(func() {
							outbox.Clear()
							clock.Fire()
							Eventually(le).ShouldNot(BeZero())
						})

						It("sends the invitation after ~4 hours", func() {
							Ω(clock.Time()).Should(BeOn(time.Tuesday, 10, testConfig.Offset))
							Ω(le()).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("Please let me know if you'll be joining us this Saturday " + gameDate)))
							Ω(le()).Should(HaveText(ContainSubstring("Where: James Bible Park")))
							Ω(le()).Should(HaveText(ContainSubstring("Weather Forecast: 🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph")))
							Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>Where</strong>: <a href="https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9" target="_blank">James Bible Park</a>`)))
							Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
						})

						Context("if the boss then replies", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
								Eventually(le).ShouldNot(BeZero())
							})

							It("tells the boss they're too late", func() {
								Ω(le()).Should(HaveSubject("Re: [invite-approval-request] Can I send this week's invite?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("You sent me this e-mail, but my current state is: invite_sent")))
								Ω(le()).Should(HaveText(ContainSubstring("> /approve")))
								Ω(le()).Should(HaveHTML(BeEmpty()))
								Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
							})
						})
					})

					Context("if the boss asks for a delay", func() {
						Context("and the delay is malformed", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay"))
							})

							It("returns an error and doesn't change the timer", func() {
								Eventually(le).ShouldNot(BeZero())
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, /no, /delay <int>, /abort, or /RESET-RESET-RESET")))

								outbox.Clear()
								clock.Fire()
								Eventually(le).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
								Ω(clock.Time()).Should(BeOn(time.Tuesday, 10, testConfig.Offset))
							})
						})

						Context("and the delay is for 0 hours", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay 0"))
							})

							It("returns an error and doesn't change the timer", func() {
								Eventually(le).ShouldNot(BeZero())
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("invalid delay count for /delay command: 0 - must be > 0")))

								outbox.Clear()
								clock.Fire()
								Eventually(le).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
								Ω(clock.Time()).Should(BeOn(time.Tuesday, 10, testConfig.Offset))
							})
						})

						Context("and the delay is for some positive number of hours", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay 1"))
							})

							It("acknowledges the request and delays sending the invite by that many hours", func() {
								Eventually(le).ShouldNot(BeZero())
								Ω(le()).Should(HaveSubject("Re: [invite-approval-request] Can I send this week's invite?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("I've delayed sending the invite email by 1 hours")))

								outbox.Clear()
								clock.Fire()
								Eventually(le).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
								Ω(clock.Time()).Should(BeOn(time.Tuesday, 11, testConfig.Offset)) // 1 hour later
							})
						})
					})

					Context("if the boss replies in the affirmative", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
							Eventually(le).ShouldNot(BeZero())
						})

						It("sends the invitation immediately", func() {
							Ω(le()).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("Please let me know if you'll be joining us this Saturday " + gameDate)))
							Ω(le()).Should(HaveHTML(ContainSubstring("<strong>")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
						})
					})

					Context("if the boss replies with /abort", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/abort"))
							Eventually(le).ShouldNot(BeZero())
						})

						It("does not send the invitation and sends an abort e-mail", func() {
							Ω(le()).Should(HaveSubject("Re: [invite-approval-request] Can I send this week's invite?"))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("Alright.  I'm aborting.  You're on the hook for keeping eyes on things.")))

							Consistently(outbox.Emails).Should(HaveLen(1))
							Ω(disco.GetSnapshot()).Should(HaveState(StateAbort))
						})
					})

					Context("if the boss replies in the affirmative with an additional message", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nLets **GO!**"))
							Eventually(le).ShouldNot(BeZero())
						})

						It("sends the invitation immediately", func() {
							Ω(le()).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("Lets GO!")))
							Ω(le()).Should(HaveText(ContainSubstring("Please let me know if you'll be joining us this Saturday " + gameDate)))
							Ω(le()).Should(HaveHTML(ContainSubstring("Lets <strong>GO!</strong>")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
						})
					})

					Context("if the boss replies in the negative", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
							Eventually(le).ShouldNot(BeZero())
						})

						It("sends the no-invitation immediately", func() {
							Ω(le()).Should(HaveSubject("No Saturday Bible Park Frisbee This Week"))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("No Saturday game this week.  We'll try again next week!")))
							Ω(le()).Should(HaveHTML(ContainSubstring("<a href")))
						})

						It("resets when the clock next ticks, on Saturday at noon", func() {
							Ω(disco.GetSnapshot()).Should(HaveState(StateNoInviteSent))
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Saturday, 12, testConfig.Offset))
							Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
						})
					})

					Context("if the boss replies in the negative with an additional message", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no\n\nOn account of **weather**...\n\n:("))
							Eventually(le).ShouldNot(BeZero())
						})

						It("sends the no-invitation immediately", func() {
							Ω(le()).Should(HaveSubject("No Saturday Bible Park Frisbee This Week"))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(ContainSubstring("On account of weather...\n\n:(\n\nNo Saturday game this week.  We'll try again next week!")))
							Ω(le()).Should(HaveHTML(ContainSubstring("On account of <strong>weather</strong>&hellip;")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateNoInviteSent))
						})
					})
				})

				Describe("handling malformed commands and replies", func() {
					Context("when the reply subject doesn't match", func() {
						It("tells the boss", func() {
							bossToDisco("Re: [floop] hey", "/approve")
							Eventually(le).Should(HaveSubject("Re: [floop] hey"))
							Ω(le()).Should(HaveText(ContainSubstring("invalid reply subject: Re: [floop] hey")))
						})
					})

					Context("when the reply body doesn't match a known command", func() {
						It("tells the boss", func() {
							bossToDisco("Re: [invite-approval-request] hey", "/yeppers")
							Eventually(le).Should(HaveSubject("Re: [invite-approval-request] hey"))
							Ω(le()).Should(HaveText(ContainSubstring("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, /no, /delay <int>, /abort, or /RESET-RESET-RESET")))
						})
					})
				})

				Describe("badgering users: after the invite is sent - when there is no quorum yet", func() {
					var approvalRequest mail.Email
					BeforeEach(func() {
						Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateInviteSent))
						bossToDisco("/set player@example.com 7")
						Eventually(disco.GetSnapshot).Should(HaveCount(7)) // no quorum yet

						outbox.Clear()
						clock.Fire()
						Eventually(le).ShouldNot(BeZero())
						approvalRequest = le()
					})

					It("asks for permission to badger users on Thursday at 10am", func() {
						Ω(clock.Time()).Should(BeOn(time.Thursday, 14, testConfig.Offset))
						Ω(le()).Should(HaveSubject("[badger-approval-request] Can I badger folks?"))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
						Ω(le()).Should(HaveText(ContainSubstring("Ignore this e-mail to have me send the badger eventually (on %s)", clock.Time().Add(4*time.Hour).Format("Monday 1/2 3:04pm"))))
						Ω(le()).Should(HaveText(ContainSubstring("--- Badger Email ---\nSubject: Last Call! " + gameDate)))
						Ω(le()).Should(HaveHTML(BeEmpty()))
						Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedBadgerApproval))
					})

					Context("if there is still no quorum after time has elapsed", func() {
						Context("if the boss doesn't reply", func() {
							BeforeEach(func() {
								outbox.Clear()
								clock.Fire()
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the badger after ~4 hours", func() {
								Ω(clock.Time()).Should(BeOn(time.Thursday, 18, testConfig.Offset))
								Ω(le()).Should(HaveSubject("Last Call! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We're still short.  Anyone forget to respond?")))
								Ω(le()).Should(HaveText(ContainSubstring("player (7)")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
							})

							It("checks for quorum on Friday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							})

							Context("if the boss then replies", func() {
								BeforeEach(func() {
									outbox.Clear()
									handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
									Eventually(le).ShouldNot(BeZero())
								})

								It("tells the boss they're too late", func() {
									Ω(le()).Should(HaveSubject("Re: [badger-approval-request] Can I badger folks?"))
									Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
									Ω(le()).Should(BeSentTo(conf.BossEmail))
									Ω(le()).Should(HaveText(ContainSubstring("You sent me this e-mail, but my current state is: badger_sent")))
									Ω(le()).Should(HaveText(ContainSubstring("> /approve")))
									Ω(le()).Should(HaveHTML(BeEmpty()))
									Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
								})
							})
						})

						Context("if the boss replies in the affirmative", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the badger immediately", func() {
								Ω(le()).Should(HaveSubject("Last Call! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We're still short.  Anyone forget to respond?")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
							})

							It("checks for quorum on Friday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							})

						})

						Context("if the boss replies in the affirmative with an additional message", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nWe only have **FIVE**."))
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the badger immediately", func() {
								Ω(le()).Should(HaveSubject("Last Call! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We only have FIVE.")))
								Ω(le()).Should(HaveText(ContainSubstring("We're still short.  Anyone forget to respond?")))
								Ω(le()).Should(HaveHTML(ContainSubstring("We only have <strong>FIVE</strong>.")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
							})

							It("checks for quorum on Friday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							})
						})

						Context("if the boss asks for a delay", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay 1"))
							})

							It("acknowledges the request and delays sending the invite by that many hours", func() {
								Eventually(le).ShouldNot(BeZero())
								Ω(le()).Should(HaveSubject("Re: [badger-approval-request] Can I badger folks?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("I've delayed sending the badger email by 1 hours")))

								outbox.Clear()
								clock.Fire()
								Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
								Ω(clock.Time()).Should(BeOn(time.Thursday, 19, testConfig.Offset)) // 1 hour later
							})
						})

						Context("if the boss replies in the negative", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
								Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
							})

							It("sends no e-mail", func() {
								Consistently(le).Should(BeZero())
							})

							It("checks for quorum on Friday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							})
						})
					})

					Context("if quorum is attained before the boss replies or the badger is sent", func() {
						BeforeEach(func() {
							clock.SetTime(clock.Time().Add(time.Hour)) // an hour later, i.e. the badger hasn't triggered yet
							outbox.Clear()
							bossToDisco("/set onsijoe@gmail.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
						})

						Context("when its time to send the badger", func() {
							BeforeEach(func() {
								outbox.Clear()
								clock.Fire()
							})

							It("sends the game-on approval email instead", func() {
								Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedGameOnApproval))
								Ω(clock.Time()).Should(BeOn(time.Thursday, 18, testConfig.Offset)) //the auto-badger time
								Ω(le()).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
								Ω(le()).Should(HaveText(ContainSubstring("Ignore this e-mail to have me send the game on eventually (on %s)", clock.Time().Add(4*time.Hour).Format("Monday 1/2 3:04pm"))))
								Ω(le()).Should(HaveText(ContainSubstring("--- Game On Email ---\nSubject: GAME ON THIS SATURDAY! " + gameDate)))
								Ω(le()).Should(HaveHTML(BeEmpty()))
							})
						})

						Context("when the boss approves the badger", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
								Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
							})

							It("sends the badger anyway", func() {
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We're still short.  Anyone forget to respond?")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
							})

							It("checks for quorum on Friday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							})
						})
					})

					Describe("if, after the badger is sent, there is quorum", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
							Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
							bossToDisco("/set onsijoe@gmail.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
						})

						It("requests game-on approval on Friday morning", func() {
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							Eventually(le).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
						})
					})

					Describe("if, after the badger is sent, there is still no quorum", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
							Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
						})

						It("requests no-game approval on Friday morning", func() {
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							Eventually(le).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedNoGameApproval))
						})
					})

					Describe("if, the badger is not sent, but there comes to be quorum", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/deny"))
							Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
							bossToDisco("/set onsijoe@gmail.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
						})

						It("requests game-on approval on Friday morning", func() {
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							Eventually(le).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
						})
					})

					Describe("if, the badger is not sent and there is still no quorum", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/deny"))
							Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
						})

						It("requests no-game approval on Friday morning", func() {
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Friday, 6, testConfig.Offset))
							Eventually(le).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedNoGameApproval))
						})
					})
				})

				Describe("after the invite is sent - when there is quorum", func() {
					var approvalRequest mail.Email
					BeforeEach(func() {
						Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateInviteSent))
						bossToDisco("/set player@example.com 5")
						Eventually(disco.GetSnapshot).Should(HaveCount(5))
						bossToDisco("/set Onsi Fakhouri <onsijoe@gmail.com> 1")
						Eventually(disco.GetSnapshot).Should(HaveCount(6))
						bossToDisco("/set Josh McJoshson <josh@example.com> 2")
						Eventually(disco.GetSnapshot).Should(HaveCount(8)) //quorum!

						outbox.Clear()
						clock.Fire()
						Eventually(le).ShouldNot(BeZero())
						approvalRequest = le()
					})

					It("asks for permission to call game on Thursday at 10am", func() {
						Ω(clock.Time()).Should(BeOn(time.Thursday, 14, testConfig.Offset))
						Ω(le()).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
						Ω(le()).Should(HaveText(ContainSubstring("Ignore this e-mail to have me send the game on eventually (on %s)", clock.Time().Add(4*time.Hour).Format("Monday 1/2 3:04pm"))))
						Ω(le()).Should(HaveText(ContainSubstring("--- Game On Email ---\nSubject: GAME ON THIS SATURDAY! " + gameDate)))
						Ω(le()).Should(HaveHTML(BeEmpty()))
						Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
					})

					Context("if there is still quorum after time has elapsed", func() {
						Context("if the boss doesn't reply", func() {
							BeforeEach(func() {
								outbox.Clear()
								clock.Fire()
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the game on e-mail after ~4 hours", func() {
								Ω(clock.Time()).Should(BeOn(time.Thursday, 18, testConfig.Offset))
								Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We have quorum!  GAME ON for " + gameDate)))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
							})

							It("sends a reminder on Saturday morning and then resets", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Saturday, 6, testConfig.Offset))
								Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))

								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Saturday, 12, testConfig.Offset))
								Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
							})

							Context("if the boss then replies", func() {
								BeforeEach(func() {
									outbox.Clear()
									handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
									Eventually(le).ShouldNot(BeZero())
								})

								It("tells the boss they're too late", func() {
									Ω(le()).Should(HaveSubject("Re: [game-on-approval-request] Can I call GAME ON?"))
									Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
									Ω(le()).Should(BeSentTo(conf.BossEmail))
									Ω(le()).Should(HaveText(ContainSubstring("You sent me this e-mail, but my current state is: game_on_sent")))
									Ω(le()).Should(HaveText(ContainSubstring("> /approve")))
									Ω(le()).Should(HaveHTML(BeEmpty()))
									Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
								})
							})

						})

						Context("if the boss replies in the affirmative", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the game-on email immediately", func() {
								Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We have quorum!  GAME ON for " + gameDate)))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
							})

							It("sends a reminder on Saturday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Saturday, 6, testConfig.Offset))
								Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Total</strong>: 8 🎉")))
							})
						})

						Context("if the boss replies in the affirmative with an additional message", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nWe have a **solid** group this week!"))
								Eventually(le).ShouldNot(BeZero())
							})

							It("sends the game-on email immediately", func() {
								Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("We have quorum!")))
								Ω(le()).Should(HaveText(ContainSubstring("We have a solid group this week!")))
								Ω(le()).Should(HaveHTML(ContainSubstring("We have a <strong>solid</strong> group this week!")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
							})

							It("sends a reminder on Saturday morning", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Saturday, 6, testConfig.Offset))
								Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
								Ω(le()).Should(HaveHTML(ContainSubstring("<strong>Players</strong>: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
							})
						})

						Context("if the boss asks for a delay", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay 1"))
							})

							It("acknowledges the request and delays sending the invite by that many hours", func() {
								Eventually(le).ShouldNot(BeZero())
								Ω(le()).Should(HaveSubject("Re: [game-on-approval-request] Can I call GAME ON?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("I've delayed sending the game on email by 1 hours")))

								outbox.Clear()
								clock.Fire()
								Eventually(le).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
								Ω(clock.Time()).Should(BeOn(time.Thursday, 19, testConfig.Offset)) // 1 hour later
							})
						})

						Context("if the boss replies in the negative", func() {
							BeforeEach(func() {
								outbox.Clear()
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no\nWe have the numbers but the **weather** has turned :("))
								Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
							})

							It("sends the no-game email", func() {
								Ω(le()).Should(HaveSubject("No Saturday Game This Week " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(ContainSubstring("No Saturday game this week.  We'll try again next week!")))
								Ω(le()).Should(HaveHTML(ContainSubstring("the <strong>weather</strong> has turned")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
							})

							It("resets when the clock next ticks, on Saturday at noon", func() {
								Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Saturday, 12, testConfig.Offset))
								Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
							})
						})
					})

					Context("if quorum is lost before its time", func() {
						BeforeEach(func() {
							bossToDisco("/set Onsi Fakhouri <onsijoe@gmail.com> 0")
							Eventually(disco.GetSnapshot).Should(HaveCount(7))
						})

						Context("and the boss doesn't reply before the timer goes off", func() {
							BeforeEach(func() {
								clock.Fire()
								Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedNoGameApproval))
							})

							It("asks for permission to send the no-game email", func() {
								Ω(clock.Time()).Should(BeOn(time.Thursday, 18, testConfig.Offset))
								Ω(le()).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("Respond with /deny or /no **to abort this week**")))
								Ω(le()).Should(HaveText(ContainSubstring("Ignore this e-mail to have me send the no game eventually (on %s)", clock.Time().Add(4*time.Hour).Format("Monday 1/2 3:04pm"))))
								Ω(le()).Should(HaveHTML(""))
							})
						})

						Context("and the boss approves", func() {
							BeforeEach(func() {
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes"))
								Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedNoGameApproval))
							})

							It("replies to the approval with an error - quorum was lost, then sends a no-game request and enters the NoGameApproval flow", func() {
								numEmails := len(outbox.Emails())
								errorEmail := outbox.Emails()[numEmails-2]
								noGameApprovalEmail := outbox.Emails()[numEmails-1]

								Ω(errorEmail).Should(HaveSubject(HavePrefix("Re: [game-on-approval-request]")))
								Ω(errorEmail).Should(BeSentTo(conf.BossEmail))
								Ω(errorEmail).Should(HaveText(ContainSubstring("Quorum was lost before this approval came in.  Starting the No-Game flow soon.")))

								Ω(noGameApprovalEmail).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
								Ω(noGameApprovalEmail).Should(BeSentTo(conf.BossEmail))
								Ω(noGameApprovalEmail).Should(HaveText(ContainSubstring("Respond with /deny or /no **to abort this week**")))
							})
						})

						Context("and the boss declines", func() {
							BeforeEach(func() {
								handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
								Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
							})

							It("sends the no-game email", func() {
								Ω(le()).Should(HaveSubject("No Saturday Game This Week " + gameDate))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
								Ω(le()).Should(HaveText(HavePrefix("No Saturday game this week.  We'll try again next week!")))
								Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
							})
						})
					})
				})

				Describe("the no game flow", func() {
					var approvalRequest mail.Email
					BeforeEach(func() {
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedInviteApproval))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateInviteSent))
						bossToDisco("/set onsijoe@gmail.com 7") //almost quorum! but not quite
						Eventually(disco.GetSnapshot).Should(HaveCount(7))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedBadgerApproval))
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerSent))
						outbox.Clear()
						clock.Fire()
						Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedNoGameApproval))
						approvalRequest = le()
					})

					It("asks for permission to send the no-game email", func() {
						Ω(approvalRequest).Should(BeSentTo(conf.BossEmail))
						Ω(approvalRequest).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
						Ω(approvalRequest).Should(HaveText(ContainSubstring("Respond with /deny or /no **to abort this week**")))
						Ω(approvalRequest).Should(HaveHTML(""))
					})

					Context("if the boss doesn't respond in time", func() {
						BeforeEach(func() {
							clock.Fire()
						})

						It("sends the no-game e-mail", func() {
							Ω(clock.Time()).Should(BeOn(time.Friday, 10, testConfig.Offset))
							Eventually(le).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(HavePrefix("No Saturday game this week.  We'll try again next week!")))
							Ω(le()).Should(HaveText(ContainSubstring("onsijoe (7)")))
							Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
						})

						It("resets on Saturday", func() {
							Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Saturday, 12, testConfig.Offset))
							Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
						})
					})

					Context("if the boss approves", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes"))
							Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
						})

						It("sends the no-game e-mail", func() {
							Eventually(le).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(HavePrefix("No Saturday game this week.  We'll try again next week!")))
						})
					})

					Context("if the boss approves with additional content", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes\nWe did **not** manage to get to quorum."))
							Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
						})

						It("sends the no-game e-mail", func() {
							Eventually(le).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
							Ω(le()).Should(HaveText(HavePrefix("We did not manage to get to quorum.\n\nNo Saturday game this week.  We'll try again next week!")))
							Ω(le()).Should(HaveHTML(ContainSubstring("We did <strong>not</strong> manage to get to quorum.")))
						})
					})

					Context("if the boss disapproves", func() {
						BeforeEach(func() {
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
							Eventually(disco.GetSnapshot).Should(HaveState(StateAbort))
						})

						It("tells the boss it's aborting and aborts", func() {
							Eventually(le).Should(HaveSubject("Re: [no-game-approval-request] Can I call NO GAME?"))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("Alright.  I'm aborting.  You're on the hook for keeping eyes on things.")))
						})
					})

					Context("if the boss asks for a delay", func() {
						BeforeEach(func() {
							outbox.Clear()
							handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/delay 1"))
						})

						It("acknowledges the request and delays sending the invite by that many hours", func() {
							Eventually(le).ShouldNot(BeZero())
							Eventually(le).Should(HaveSubject("Re: [no-game-approval-request] Can I call NO GAME?"))
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("I've delayed sending the no game email by 1 hours")))

							outbox.Clear()
							clock.Fire()
							Eventually(le).Should(HaveSubject("No Saturday Game This Week " + gameDate))
							Ω(clock.Time()).Should(BeOn(time.Friday, 11, testConfig.Offset)) // 1 hour later
						})
					})

					Context("quorum is attained before no-game is resolved", func() {
						BeforeEach(func() {
							bossToDisco("/set player@example.com 1")
							Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
						})

						Context("if the boss doesn't respond in time", func() {
							It("sends the game-on approval request", func() {
								clock.Fire()
								Ω(clock.Time()).Should(BeOn(time.Friday, 10, testConfig.Offset))
								Eventually(le).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
								Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
								Ω(le()).Should(BeSentTo(conf.BossEmail))
								Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
								Ω(le()).Should(HaveText(ContainSubstring("--- Game On Email ---\nSubject: GAME ON THIS SATURDAY! " + gameDate)))
								Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
							})
						})

						for _, response := range []string{"/yes", "/no"} {
							response := response
							Context("if the boss responds with "+response, func() {
								BeforeEach(func() {
									handleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, response))
									Eventually(disco.GetSnapshot).Should(HaveState(StateRequestedGameOnApproval))
								})

								It("replies to the approval with an error - quorum was gained, then sends a game-on request and enters the NoGameApproval flow", func() {
									numEmails := len(outbox.Emails())
									errorEmail := outbox.Emails()[numEmails-2]
									gameOnApprovalRequest := outbox.Emails()[numEmails-1]

									Ω(errorEmail).Should(HaveSubject(HavePrefix("Re: [no-game-approval-request]")))
									Ω(errorEmail).Should(BeSentTo(conf.BossEmail))
									Ω(errorEmail).Should(HaveText(ContainSubstring("Quorum was gained before this came in.  Starting the Game-On flow soon.")))

									Ω(gameOnApprovalRequest).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
									Ω(gameOnApprovalRequest).Should(BeSentTo(conf.BossEmail))
								})
							})
						}
					})
				})

				Describe("spot-checking retry logic", func() {
					Context("when an email for a scheduled event is supposed to be sent, but it fails to send", func() {
						BeforeEach(func() {
							outbox.SetError(fmt.Errorf("boom"))
							clock.Fire()
							Eventually(le).Should(HaveSubject("Help!"))
							Ω(clock.Time()).Should(BeOn(time.Tuesday, 6, testConfig.Offset))
						})

						It("sends an error email", func() {
							Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
							Ω(le()).Should(BeSentTo(conf.BossEmail))
							Ω(le()).Should(HaveText(ContainSubstring("boom")))
							Ω(le()).Should(HaveText(ContainSubstring("[invite-approval-request]"))) // the email we were trying to send is sent
							Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
						})

						It("retries in five minutes", func() {
							outbox.SetError(nil)
							clock.Fire()
							Ω(clock.Time()).Should(BeOn(time.Tuesday, 6, 5+testConfig.Offset))
							Eventually(disco.GetSnapshot()).Should(HaveState(StateRequestedInviteApproval))
							Eventually(le).Should(HaveSubject("[invite-approval-request] Can I send this week's invite?"))

						})
					})
				})
			})
		})
	}
})
