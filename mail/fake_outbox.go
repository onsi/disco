package mail

import (
	"sync"
)

type FakeOutbox struct {
	err    error
	emails []Email
	lock   *sync.Mutex
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		lock: &sync.Mutex{},
	}
}

func (o *FakeOutbox) SendEmail(email Email) error {
	o.lock.Lock()
	defer o.lock.Unlock()
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
