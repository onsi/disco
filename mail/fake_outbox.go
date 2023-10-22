package mail

import (
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/onsi/say"
)

type FakeOutbox struct {
	w      io.Writer
	err    error
	emails []Email
	lock   *sync.Mutex
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		lock: &sync.Mutex{},
		w:    io.Discard,
	}
}

func (o *FakeOutbox) EnableLogging(w io.Writer) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.w = w
}

func (o *FakeOutbox) SendEmail(email Email) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	say.Fpln(o.w, "Sending email:")
	say.Fplni(o.w, 1, "%s", email)
	email.MessageID = uuid.New().String()
	o.emails = append(o.emails, email)
	return o.err
}

func (o *FakeOutbox) SetError(err error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.err = err
}

func (o *FakeOutbox) Emails() []Email {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.emails
}

func (o *FakeOutbox) LastEmail() Email {
	o.lock.Lock()
	defer o.lock.Unlock()
	if len(o.emails) == 0 {
		return Email{}
	}
	return o.emails[len(o.emails)-1]
}

func (o *FakeOutbox) Clear() {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.emails = []Email{}
}
