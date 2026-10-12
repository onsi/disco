package saturdaydisco_test

import (
	"os"
	"strings"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	. "github.com/onsi/disco/saturdaydisco"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interpreter", func() {
	var interpreter InterpreterInt
	BeforeEach(func() {
		if os.Getenv("INCLUDE_LLM_SPECS") != "true" {
			Skip("Skipping LLM specs - use INCLUDE_LLM_SPECS=true to run them")
		}
		config := config.LoadConfig()
		Ω(config.AnthropicKey).ShouldNot(BeZero())
		interpreter = NewInterpreter(GinkgoWriter)
	})

	invitation := "Please let me know if you'll be joining us this Saturday 9/20.\n\nHow do I sign up?\n\nJust reply to this e-mail and say \"in\" if you're coming .  If you're bringing players with you say something like \"In and bringing 2 others.\"\n\nDisco 🪩"
	quoted := "On Thu, Sep 18, 2025 at 2:00 PM Saturday Disco <saturday@sedenverultimate.net> wrote:\n> " + strings.ReplaceAll(invitation, "\n", "\n> ")
	outlookQuoted := "________________________________\nFrom: Saturday Disco <saturday@sedenverultimate.net>\nSent: Thursday, September 18, 2025 2:00 PM\nSubject: Saturday Bible Park Frisbee 9/20\n\n" + invitation

	DescribeTable("it can interpret e-mails", func(body string, count int, expectedCommandType CommandType, expectedCount ...int) {
		email := mail.E().WithFrom("onsijoe@gmail.com").WithBody(body)
		actualCommand, err := interpreter.InterpretEmail(email, count)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(actualCommand.CommandType).Should(Equal(expectedCommandType))
		if len(expectedCount) > 0 {
			Ω(actualCommand.Count).Should(Equal(expectedCount[0]))
		}
		Ω(actualCommand.Email).Should(Equal(email))
		Ω(actualCommand.EmailAddress).Should(Equal(email.From))
	},
		//new players
		Entry(nil, "In!", 0, CommandPlayerSetCount, 1),
		Entry(nil, "Joseph and I can join", 0, CommandPlayerSetCount, 2),
		Entry(nil, "I'm out, sorry", 0, CommandPlayerSetCount, 0),
		Entry(nil, "out", 0, CommandPlayerSetCount, 0),
		Entry(nil, "I'm bringing the whole family (all five of us),.", 0, CommandPlayerSetCount, 5),
		Entry(nil, "I can't this week.  But I'm on for next week!", 0, CommandPlayerSetCount, 0),
		Entry(nil, "John in", 0, CommandPlayerSetCount, 1),
		Entry(nil, "no, julie can still make it", 0, CommandPlayerSetCount, 1),
		Entry(nil, "+1", 0, CommandPlayerSetCount, 1),
		Entry(nil, "in +2", 0, CommandPlayerSetCount, 3),
		Entry(nil, "Count me out but Sam's in", 0, CommandPlayerSetCount, 1),
		Entry(nil, "In!\n\n"+quoted, 0, CommandPlayerSetCount, 1),
		Entry(nil, "Can't make it this week, sorry\n\n"+quoted, 0, CommandPlayerSetCount, 0),
		Entry(nil, "in\n\n"+outlookQuoted, 0, CommandPlayerSetCount, 1),

		//updating counts
		Entry(nil, "I can't make it anymore :(  Sorry", 1, CommandPlayerSetCount, 0),
		Entry(nil, "out now, sorry", 1, CommandPlayerSetCount, 0),
		Entry(nil, "ugh. a meeting came up and i'm gonna have to bail", 1, CommandPlayerSetCount, 0),
		Entry(nil, "John can't any more", 5, CommandPlayerSetCount, 4),
		Entry(nil, "Both boys are out this week, but I can still come", 3, CommandPlayerSetCount, 1),
		Entry(nil, "Sorry, something's come up and none of us can make it any more", 3, CommandPlayerSetCount, 0),
		Entry(nil, "My cousin's in town and can join me too!", 1, CommandPlayerSetCount, 2),
		Entry(nil, "Actually my sister's coming too\n\n"+quoted, 2, CommandPlayerSetCount, 3),

		//banter
		Entry(nil, "Last week was amazing.\n\nI've planning to hand out on Thursday anybody want to join?", 0, CommandPlayerIgnore),
		Entry(nil, "By the way, there's an ultimate game showing on ESPN 7.  Anybody interested?", 0, CommandPlayerIgnore),
		Entry(nil, "Thanks for organizing!\n\n"+quoted, 1, CommandPlayerIgnore),
		Entry(nil, "Maybe - I'll know by Friday", 0, CommandPlayerIgnore),
	)
})
