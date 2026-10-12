package lunchtimedisco_test

import (
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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

var app atomic.Pointer[echo.Echo]
var appURL string

var _ = SynchronizedBeforeSuite(func() {
	biloba.SpinUpChrome(GinkgoT())
}, func() {
	b = biloba.ConnectToChrome(GinkgoT())

	// Held for the whole run so the origin stays stable and its OS-assigned port can't be taken by a concurrent run; each spec swaps in its own app.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	Ω(err).ShouldNot(HaveOccurred())
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := app.Load(); e != nil {
			e.ServeHTTP(w, r)
			return
		}
		http.Error(w, "no spec is running", http.StatusServiceUnavailable)
	})}
	go srv.Serve(l)
	DeferCleanup(srv.Close)
	appURL = "http://" + l.Addr().String()
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
