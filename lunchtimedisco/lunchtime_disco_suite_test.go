package lunchtimedisco_test

import (
	"testing"
	"time"

	"github.com/onsi/biloba"
	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLunchtimedisco(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lunchtimedisco Suite")
}

var b *biloba.Biloba

var _ = SynchronizedBeforeSuite(func() {
	biloba.SpinUpChrome(GinkgoT())
}, func() {
	b = biloba.ConnectToChrome(GinkgoT())
})

var _ = BeforeEach(func() {
	b.Prepare()
}, OncePerOrdered)

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
		for _, recipient := range e.Recipients() {
			actual = append(actual, recipient.Address())
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

func HaveState(state lunchtimedisco.LunchtimeDiscoState) OmegaMatcher {
	return HaveField("State", state)
}
func HaveGameCount(gameKey string, count int) OmegaMatcher {
	return WithTransform(func(snapshot lunchtimedisco.LunchtimeDiscoSnapshot) int {
		out := 0
		for _, participant := range snapshot.Participants {
			for _, key := range participant.GameKeys {
				if key == gameKey {
					out++
					break
				}
			}
		}
		return out
	}, Equal(count))
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
