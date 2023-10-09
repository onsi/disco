package lunchtimedisco_test

import (
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clockpkg "github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	. "github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
)

var _ = Describe("LunchtimeInterpreter", func() {
	var interpreter LunchtimeInterpreterInt
	BeforeEach(func() {
		if os.Getenv("INCLUDE_OPENAI_SPECS") != "true" {
			Skip("Skipping OpenAI specs - use INCLUDE_OPENAI_SPECS=true to run them")
		}
		config := config.LoadConfig()
		Ω(config.OpenAIKey).ShouldNot(BeZero())
		interpreter = NewLunchtimeInterpreter()
	})

	DescribeTable("it can interpret e-mails", func(body string, currentGameKeys string, expectedCommandType CommandType, expectedGameKeys ...string) {
		now := time.Date(2023, time.September, 24, 0, 0, 0, 0, clockpkg.Timezone) // a Sunday
		T := clockpkg.NextSaturdayAt10(now)
		// this means Tuesday is 9/26, Wednesday is 9/27, Thursday is 9/28, Friday is 9/29
		email := mail.E().WithFrom("onsijoe@gmail.com").WithBody(body)
		actualCommand, err := interpreter.InterpretEmail(email, T, currentGameKeys)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(actualCommand.CommandType).Should(Equal(expectedCommandType))
		if len(expectedGameKeys) > 0 {
			actual := strings.ReplaceAll(strings.TrimSpace(actualCommand.GameKeyInput), " ", "")
			actual = strings.ToUpper(actual)

			Ω(actual).Should(Equal(expectedGameKeys[0]))
		}
		Ω(actualCommand.Email).Should(Equal(email))
		Ω(actualCommand.EmailAddress).Should(Equal(email.From))
	},
		//status
		Entry(nil, "Status", "", CommandPlayerStatus),
		Entry(nil, "Is the game happening?", "", CommandPlayerStatus),
		Entry(nil, "Yo - are we playing this week or not?", "", CommandPlayerStatus),

		//new players
		Entry(nil, "I can do A, B, and D", "", CommandPlayerSetGames, "A,B,D"),
		Entry(nil, "I can do all day 9/27 and 9/29 before noon", "", CommandPlayerSetGames, "E,F,G,H,M,N"),
		Entry(nil, "I can do everything but Thursday", "", CommandPlayerSetGames, "A,B,C,D,E,F,G,H,M,N,O,P"),
		Entry(nil, "I can't this week, sorry", "", CommandPlayerSetGames, "NONE"),

		//updating counts
		Entry(nil, "I can't make it anymore :(  Sorry", "A,B,C", CommandPlayerSetGames, "NONE"),
		Entry(nil, "Can't do B any more", "A,B,C", CommandPlayerSetGames, "A,C"),
		Entry(nil, "Looks like Wednesday won't work after all", "B,F,G", CommandPlayerSetGames, "B"),

		//unsubscribe
		Entry(nil, "unsubscribe", "", CommandPlayerUnsubscribe),
		Entry(nil, "Hey - I just can't play very much these days.  Can someone take me off the list?", "", CommandPlayerUnsubscribe),
		Entry(nil, "I'm moving back east.  It's been fun y'all, but I think it's time to stop receiving these e-mails.", "", CommandPlayerUnsubscribe),

		//banter
		Entry(nil, "Last week was amazing.\n\nI've planning to hand out on Thursday anybody want to join?", "", CommandPlayerUnsure),
		Entry(nil, "By the way, there's an ultimate game showing on ESPN 7.  Anybody interested?", "", CommandPlayerUnsure),
	)
})
