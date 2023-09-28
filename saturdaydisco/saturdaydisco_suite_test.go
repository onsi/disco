package saturdaydisco_test

import (
	"testing"
	"time"

	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/saturdaydisco"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gleak"
)

func TestSaturdaydisco(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Saturdaydisco Suite")
}

var _ = BeforeSuite(func() {
	gleak.IgnoreGinkgoParallelClient()
})

var _ = BeforeEach(func() {
	DeferCleanup(func() {
		Eventually(gleak.Goroutines).ShouldNot(gleak.HaveLeaked())
	})
})

func BeFrom(sender mail.EmailAddress) OmegaMatcher {
	return WithTransform(func(e mail.Email) string {
		return e.From.Address()
	}, Equal(sender.Address()))
}

func BeSentTo(recipients ...mail.EmailAddress) OmegaMatcher {
	expected := make([]string, len(recipients))
	for i, r := range recipients {
		expected[i] = r.Address()
	}
	return WithTransform(func(e mail.Email) []string {
		actual := []string{}
		for _, t := range e.To {
			actual = append(actual, t.Address())
		}
		for _, cc := range e.CC {
			actual = append(actual, cc.Address())
		}
		return actual
	}, ConsistOf(expected))
}

func HaveSubject(subject any) OmegaMatcher {
	return HaveField("Subject", subject)
}

func HaveText(text any) OmegaMatcher {
	return HaveField("Text", text)
}

func HaveHTML(html any) OmegaMatcher {
	return HaveField("HTML", html)
}

func HaveState(state saturdaydisco.SaturdayDiscoState) OmegaMatcher {
	return HaveField("State", state)
}

func BeOn(day time.Weekday, hour int, optionalMinute ...int) OmegaMatcher {
	minute := 0
	if len(optionalMinute) > 0 {
		minute = optionalMinute[0]
	}

	type onTime struct {
		Day    time.Weekday
		Hour   int
		Minute int
	}

	expected := onTime{
		Day:    day,
		Hour:   hour,
		Minute: minute,
	}

	return WithTransform(func(t time.Time) onTime {
		return onTime{
			Day:    t.Weekday(),
			Hour:   t.Hour(),
			Minute: t.Minute(),
		}
	}, Equal(expected))
}
