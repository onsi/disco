package saturdaydisco_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	. "github.com/onsi/disco/saturdaydisco"
)

/*
TODO:

- test the commands
- think about behavior when aborted.  becomes completely command driven?
- spot-check errors in the e-mail command parser

- before error testing, commit and then step back.  is there a cleaner way to express the state machine?


- spot-check retries for state machine
- spot-check retries for e-mail commands
*/

var _ = Describe("SaturdayDisco", func() {
	var outbox *mail.FakeOutbox
	var clock *FakeAlarmClock
	var disco *SaturdayDisco
	var conf config.Config

	var now time.Time
	var gameDate string

	var le func() mail.Email
	var bossToDisco = func(args ...string) {
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
		disco.HandleIncomingEmail(mail.E().
			WithFrom(conf.BossEmail).
			WithTo(conf.SaturdayDiscoEmail).
			WithSubject(subject).
			WithBody(body))
	}

	BeforeEach(func() {
		outbox = mail.NewFakeOutbox()
		le = outbox.LastEmail
		clock = NewFakeAlarmClock()
		conf.BossEmail = mail.EmailAddress("Boss <boss@example.com>")
		conf.SaturdayDiscoEmail = mail.EmailAddress("Disco <saturday-disco@sedenverultimate.net>")
		conf.SaturdayDiscoList = mail.EmailAddress("Saturday-List <saturday-se-denver-ultimate@googlegroups.com>")

		now = time.Date(2023, time.September, 24, 0, 0, 0, 0, Timezone) // a Sunday
		gameDate = "9/30/23"                                            //the following Saturday
		clock.SetTime(now)

		disco = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox)
		DeferCleanup(disco.Stop)
		Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
	})

	Describe("commands", func() {
		Describe("when an unknown command is sent by the boss", func() {
			XIt("replies with an error e-mail", func() {

			})
		})

		Describe("when a player sends an e-mail just to disco and disco is unsure", func() {
			XIt("replies and CCs the boss", func() {

			})
		})

		Describe("when the boss send an e-mail that includes the list", func() {
			It("totally ignores the boss' email, even if its a valid command", func() {

			})
		})

		Describe("when a player sends an e-mail that includes the list", func() {
			Context("and disco thinks its a command", func() {
				It("replies and CCs the boss", func() {

				})
			})

			Context("and disco is unsure", func() {
				It("does nothing", func() {

				})
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
					Ω(le()).Should(HaveRecipients(ConsistOf(conf.BossEmail)))
					Ω(le()).Should(HaveText(ContainSubstring("I've set player@example.com to 4")))
					Ω(le()).Should(HaveText(ContainSubstring("Current State: invite_sent")))
					Ω(le()).Should(HaveText(ContainSubstring("Onsi Fakhouri <onsijoe@gmail.com>: 2")))
					Ω(le()).Should(HaveText(ContainSubstring("player@example.com: 4")))
					Ω(le()).Should(HaveText(ContainSubstring("Total Count: 6")))
					Ω(le()).Should(HaveText(ContainSubstring("Has Quorum: false")))
				})

				It("allows the admin to change a players' count", func() {
					bossToDisco("/set onsijoe@gmail.com 2")
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
					Ω(le()).Should(HaveText(ContainSubstring("Onsi Fakhouri <onsijoe@gmail.com>: 6"))) // we got the name!

				})

				It("send back an error if the admin messes up", func() {
					bossToDisco("/set")
					Eventually(le).Should(HaveText(ContainSubstring("could not extract valid command from: /set")))
					bossToDisco("/set 2")
					Eventually(le).Should(HaveText(ContainSubstring("could not extract valid command from: /set 2")))
					bossToDisco("/set onsijoe@gmail.com two")
					Eventually(le).Should(HaveText(ContainSubstring("could not extract valid command from: /set onsijoe@gmail.com two")))
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
					disco.HandleIncomingEmail(mail.E().
						WithFrom(conf.BossEmail).
						WithTo(conf.SaturdayDiscoList).
						WithSubject("hey").
						WithBody("/set onsijoe@gmail.com 2"))
					Consistently(le).Should(BeZero())
				})
			})

			XDescribe("the user-facing interface for setting players", func() {
				It("allows players to register themselves, replying only to them and CCing boss", func() {

				})

				It("allows players to change their count, replying only to them and CCing boss", func() {

				})
			})
		})

		Describe("getting status", func() {
			Describe("the boss' interface", func() {
				BeforeEach(func() {
					clock.Fire() // invite approval
					clock.Fire() // invite
					bossToDisco("/set player@example.com 1")
					Eventually(disco.GetSnapshot).Should(HaveCount(1))
					bossToDisco("/set onsijoe@gmail.com 2")
					Eventually(disco.GetSnapshot).Should(HaveCount(3))
					outbox.Clear()
					bossToDisco("/status")
					Eventually(le).Should(HaveSubject("Re: hey"))
				})

				It("returns status", func() {
					Ω(le()).Should(BeSentTo(conf.BossEmail))
					Ω(le()).Should(HaveText(ContainSubstring("Current State: invite_sent")))
					Ω(le()).Should(HaveText(ContainSubstring("Participants:")))
					Ω(le()).Should(HaveText(ContainSubstring("- player@example.com: 1")))
					Ω(le()).Should(HaveText(ContainSubstring("- onsijoe@gmail.com: 2")))
					Ω(le()).Should(HaveText(ContainSubstring("Total Count: 3")))
					Ω(le()).Should(HaveText(ContainSubstring("Has Quorum: false")))

					Ω(le()).Should(HaveHTML(""))
				})
			})

			XDescribe("the player's interface", func() {
				It("allows players to get a status update, replying to all", func() {

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

		Describe("if a player wants to unsubscribe", func() {
			XIt("replies and includes boss", func() {

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
				Ω(clock.Time()).Should(BeOn(time.Tuesday, 6))
				Ω(le()).Should(HaveSubject("[invite-approval-request] Can I send this week's invite?"))
				Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
				Ω(le()).Should(BeSentTo(conf.BossEmail))
				Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
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
					Ω(clock.Time()).Should(BeOn(time.Tuesday, 10))
					Ω(le()).Should(HaveSubject("Saturday Bible Park Frisbee " + gameDate))
					Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
					Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
					Ω(le()).Should(HaveText(ContainSubstring("Please let me know if you'll be joining us this Saturday " + gameDate)))
					Ω(le()).Should(HaveText(ContainSubstring("Where: James Bible Park")))
					Ω(le()).Should(HaveHTML(ContainSubstring(`<strong>Where</strong>: <a href="https://maps.app.goo.gl/P1vm2nkZdYLGZbxb9" target="_blank">James Bible Park</a>`)))
					Ω(disco.GetSnapshot()).Should(HaveState(StateInviteSent))
				})

				Context("if the boss then replies", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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

			Context("if the boss replies in the affirmative", func() {
				BeforeEach(func() {
					outbox.Clear()
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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

			Context("if the boss replies in the affirmative with an additional message", func() {
				BeforeEach(func() {
					outbox.Clear()
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nLets **GO!**"))
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
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
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
					Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
					Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
				})
			})

			Context("if the boss replies in the negative with an additional message", func() {
				BeforeEach(func() {
					outbox.Clear()
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no\n\nOn account of **weather**...\n\n:("))
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
					Ω(le()).Should(HaveText(ContainSubstring("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, or /no")))
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
				Ω(clock.Time()).Should(BeOn(time.Thursday, 14))
				Ω(le()).Should(HaveSubject("[badger-approval-request] Can I badger folks?"))
				Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
				Ω(le()).Should(BeSentTo(conf.BossEmail))
				Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
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
						Ω(clock.Time()).Should(BeOn(time.Thursday, 18))
						Ω(le()).Should(HaveSubject("Last Call! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("We're still short.  Anyone forget to respond?")))
						Ω(disco.GetSnapshot()).Should(HaveState(StateBadgerSent))
					})

					It("checks for quorum on Friday morning", func() {
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					})

					Context("if the boss then replies", func() {
						BeforeEach(func() {
							outbox.Clear()
							disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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
						Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					})

				})

				Context("if the boss replies in the affirmative with an additional message", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nWe only have **FIVE**."))
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
						Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					})

				})

				Context("if the boss replies in the negative", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
						Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
					})

					It("sends no e-mail", func() {
						Consistently(le).Should(BeZero())
					})

					It("checks for quorum on Friday morning", func() {
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Friday, 6))
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
						Ω(clock.Time()).Should(BeOn(time.Thursday, 18)) //the auto-badger time
						Ω(le()).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
						Ω(le()).Should(HaveText(ContainSubstring("--- Game On Email ---\nSubject: GAME ON THIS SATURDAY! " + gameDate)))
						Ω(le()).Should(HaveHTML(BeEmpty()))
					})
				})

				Context("when the boss approves the badger", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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
						Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					})
				})
			})

			Describe("if, after the badger is sent, there is quorum", func() {
				BeforeEach(func() {
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
					Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
					bossToDisco("/set onsijoe@gmail.com 1")
					Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
				})

				It("requests game-on approval on Friday morning", func() {
					clock.Fire()
					Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					Eventually(le).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
					Ω(le()).Should(BeSentTo(conf.BossEmail))
					Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
				})
			})

			Describe("if, after the badger is sent, there is still no quorum", func() {
				BeforeEach(func() {
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
					Eventually(le).Should(HaveSubject("Last Call! " + gameDate))
				})

				It("requests no-game approval on Friday morning", func() {
					clock.Fire()
					Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					Eventually(le).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
					Ω(le()).Should(BeSentTo(conf.BossEmail))
					Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedNoGameApproval))
				})
			})

			Describe("if, the badger is not sent, but there comes to be quorum", func() {
				BeforeEach(func() {
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/deny"))
					Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
					bossToDisco("/set onsijoe@gmail.com 1")
					Eventually(disco.GetSnapshot).Should(HaveCount(8)) // quorum!
				})

				It("requests game-on approval on Friday morning", func() {
					clock.Fire()
					Ω(clock.Time()).Should(BeOn(time.Friday, 6))
					Eventually(le).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
					Ω(le()).Should(BeSentTo(conf.BossEmail))
					Ω(disco.GetSnapshot()).Should(HaveState(StateRequestedGameOnApproval))
				})
			})

			Describe("if, the badger is not sent and there is still no quorum", func() {
				BeforeEach(func() {
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/deny"))
					Eventually(disco.GetSnapshot).Should(HaveState(StateBadgerNotSent))
				})

				It("requests no-game approval on Friday morning", func() {
					clock.Fire()
					Ω(clock.Time()).Should(BeOn(time.Friday, 6))
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
				Ω(clock.Time()).Should(BeOn(time.Thursday, 14))
				Ω(le()).Should(HaveSubject("[game-on-approval-request] Can I call GAME ON?"))
				Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
				Ω(le()).Should(BeSentTo(conf.BossEmail))
				Ω(le()).Should(HaveText(ContainSubstring("Respond with /approve")))
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
						Ω(clock.Time()).Should(BeOn(time.Thursday, 18))
						Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("We have quorum!  GAME ON for " + gameDate)))
						Ω(le()).Should(HaveHTML(ContainSubstring("Players: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
						Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
					})

					It("sends a reminder on Saturday morning and then resets", func() {
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Saturday, 6))
						Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
						Ω(le()).Should(HaveHTML(ContainSubstring("Players: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))

						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
						Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
					})

					Context("if the boss then replies", func() {
						BeforeEach(func() {
							outbox.Clear()
							disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
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
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve"))
						Eventually(le).ShouldNot(BeZero())
					})

					It("sends the game-on email immediately", func() {
						Ω(le()).Should(HaveSubject("GAME ON THIS SATURDAY! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("We have quorum!  GAME ON for " + gameDate)))
						Ω(le()).Should(HaveHTML(ContainSubstring("Players: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
						Ω(disco.GetSnapshot()).Should(HaveState(StateGameOnSent))
					})

					It("sends a reminder on Saturday morning", func() {
						clock.Fire()
						Ω(clock.Time()).Should(BeOn(time.Saturday, 6))
						Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
						Ω(le()).Should(HaveHTML(ContainSubstring("Players: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
					})
				})

				Context("if the boss replies in the affirmative with an additional message", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/approve\n\nWe have a **solid** group this week!"))
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
						Ω(clock.Time()).Should(BeOn(time.Saturday, 6))
						Eventually(le).Should(HaveSubject("Reminder: GAME ON TODAY! " + gameDate))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
						Ω(le()).Should(HaveText(ContainSubstring("Join us, we're playing today!")))
						Ω(le()).Should(HaveHTML(ContainSubstring("Players: player <strong>(5)</strong>, Onsi and Josh <strong>(2)</strong>")))
					})
				})

				Context("if the boss replies in the negative", func() {
					BeforeEach(func() {
						outbox.Clear()
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no\nWe have the numbers but the **weather** has turned :("))
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
						Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
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
						Ω(clock.Time()).Should(BeOn(time.Thursday, 18))
						Ω(le()).Should(HaveSubject("[no-game-approval-request] Can I call NO GAME?"))
						Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
						Ω(le()).Should(BeSentTo(conf.BossEmail))
						Ω(le()).Should(HaveText(ContainSubstring("Respond with /deny or /no **to abort this week**")))
						Ω(le()).Should(HaveHTML(""))
					})
				})

				Context("and the boss approves", func() {
					BeforeEach(func() {
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes"))
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
						disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
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
					Ω(clock.Time()).Should(BeOn(time.Friday, 10))
					Eventually(le).Should(HaveSubject("No Saturday Game This Week " + gameDate))
					Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
					Ω(le()).Should(BeSentTo(conf.SaturdayDiscoList))
					Ω(le()).Should(HaveText(HavePrefix("No Saturday game this week.  We'll try again next week!")))
					Ω(disco.GetSnapshot()).Should(HaveState(StateNoGameSent))
				})

				It("resets on Saturday", func() {
					Eventually(disco.GetSnapshot).Should(HaveState(StateNoGameSent))
					clock.Fire()
					Ω(clock.Time()).Should(BeOn(time.Saturday, 12))
					Eventually(disco.GetSnapshot).Should(HaveState(StatePending))
				})
			})

			Context("if the boss approves", func() {
				BeforeEach(func() {
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes"))
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
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/yes\nWe did **not** manage to get to quorum."))
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
					disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, "/no"))
					Eventually(disco.GetSnapshot).Should(HaveState(StateAbort))
				})

				It("tells the boss it's aborting and aborts", func() {
					Eventually(le).Should(HaveSubject("Re: [no-game-approval-request] Can I call NO GAME?"))
					Ω(le()).Should(BeFrom(conf.SaturdayDiscoEmail))
					Ω(le()).Should(BeSentTo(conf.BossEmail))
					Ω(le()).Should(HaveText(ContainSubstring("Alright.  I'm aborting.  You're on the hook for keeping eyes on things.")))
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
						Ω(clock.Time()).Should(BeOn(time.Friday, 10))
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
							disco.HandleIncomingEmail(approvalRequest.ReplyWithoutQuote(conf.BossEmail, response))
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
	})

})
