package saturdaydisco

import (
	"sync"

	"github.com/onsi/disco/mail"
)

type FakeInterpreter struct {
	emails        []mail.Email
	counts        []int
	returnCommand Command
	returnErr     error

	lock *sync.Mutex
}

func NewFakeInterpreter() *FakeInterpreter {
	return &FakeInterpreter{
		lock: &sync.Mutex{},
	}
}

func (interpreter *FakeInterpreter) InterpretEmail(email mail.Email, count int) (Command, error) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.emails = append(interpreter.emails, email)
	interpreter.counts = append(interpreter.counts, count)
	cmd := interpreter.returnCommand
	cmd.Email = email
	cmd.EmailAddress = email.From

	return cmd, interpreter.returnErr
}

func (interpreter *FakeInterpreter) GetEmails() []mail.Email {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	return interpreter.emails
}

func (interpreter *FakeInterpreter) GetCounts() []int {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	return interpreter.counts
}

func (interpreter *FakeInterpreter) GetMostRecentEmail() mail.Email {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	if len(interpreter.emails) == 0 {
		return mail.Email{}
	}

	return interpreter.emails[len(interpreter.emails)-1]
}

func (interpreter *FakeInterpreter) GetMostRecentCount() int {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	if len(interpreter.counts) == 0 {
		return 0
	}

	return interpreter.counts[len(interpreter.counts)-1]
}

func (interpreter *FakeInterpreter) SetCommand(command Command) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.returnCommand = command
}

func (interpreter *FakeInterpreter) SetError(err error) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.returnErr = err
}
