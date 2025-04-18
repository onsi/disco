package saturdaydisco_test

import (
	"os"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	. "github.com/onsi/disco/saturdaydisco"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interpreter", func() {
	var interpreter InterpreterInt
	BeforeEach(func() {
		if os.Getenv("INCLUDE_OPENAI_SPECS") != "true" {
			Skip("Skipping OpenAI specs - use INCLUDE_OPENAI_SPECS=true to run them")
		}
		config := config.LoadConfig()
		Ω(config.OpenAIKey).ShouldNot(BeZero())
		interpreter = NewInterpreter(GinkgoWriter)
	})

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

		//updating counts
		Entry(nil, "I can't make it anymore :(  Sorry", 1, CommandPlayerSetCount, 0),
		Entry(nil, "out now, sorry", 1, CommandPlayerSetCount, 0),
		Entry(nil, "ugh. a meeting came up and i'm gonna have to bail", 1, CommandPlayerSetCount, 0),
		Entry(nil, "John can't any more", 5, CommandPlayerSetCount, 4),
		Entry(nil, "Both boys are out this week, but I can still come", 3, CommandPlayerSetCount, 1),
		Entry(nil, "Sorry, something's come up and none of us can make it any more", 3, CommandPlayerSetCount, 0),
		Entry(nil, "My cousin's in town and can join me too!", 1, CommandPlayerSetCount, 2),

		//banter
		Entry(nil, "Last week was amazing.\n\nI've planning to hand out on Thursday anybody want to join?", 0, CommandPlayerIgnore),
		Entry(nil, "By the way, there's an ultimate game showing on ESPN 7.  Anybody interested?", 0, CommandPlayerIgnore),
	)
})
