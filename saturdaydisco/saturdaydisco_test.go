package saturdaydisco_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	. "github.com/onsi/disco/saturdaydisco"
)

/*
TODO:
- make State() a getter protected by the lock

- spot-check errors in the e-mail command parser
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

	BeforeEach(func() {
		outbox = mail.NewFakeOutbox()
		le = outbox.LastEmail
		clock = NewFakeAlarmClock()
		conf.BossEmail = mail.EmailAddress("Boss <boss@example.com>")
		conf.SaturdayDiscoEmail = mail.EmailAddress("Disco <saturday-disco@sedenverultimate.net>")
		conf.SaturdayDiscoList = mail.EmailAddress("Saturday-List <saturday-se-denver-ultimate@googlegroups.com>")

		now = time.Date(2023, time.September, 24, 0, 0, 0, 0, time.Local) // a Sunday
		gameDate = "9/30/23"                                              //the following Saturday
		clock.SetTime(now)

		disco = NewSaturdayDisco(conf, GinkgoWriter, clock, outbox)
		DeferCleanup(disco.Stop)
		Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
	})

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
				time.Sleep(time.Millisecond * 100)
				Ω(disco.GetSnapshot()).Should(HaveState(StatePending))
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
})
