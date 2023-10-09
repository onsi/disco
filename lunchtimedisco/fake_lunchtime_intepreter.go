package lunchtimedisco

import (
	"sync"
	"time"

	"github.com/onsi/disco/mail"
)

type FakeLunchtimeInterpreter struct {
	emails        []mail.Email
	ts            []time.Time
	priorGameKeys []string
	returnCommand Command
	returnErr     error

	lock *sync.Mutex
}

func NewFakeLunchtimeInterpreter() *FakeLunchtimeInterpreter {
	return &FakeLunchtimeInterpreter{
		lock: &sync.Mutex{},
	}
}

func (interpreter *FakeLunchtimeInterpreter) InterpretEmail(email mail.Email, T time.Time, priorGameKeys string) (Command, error) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.emails = append(interpreter.emails, email)
	interpreter.ts = append(interpreter.ts, T)
	interpreter.priorGameKeys = append(interpreter.priorGameKeys, priorGameKeys)
	cmd := interpreter.returnCommand
	cmd.Email = email
	cmd.EmailAddress = email.From

	return cmd, interpreter.returnErr
}

func (interpreter *FakeLunchtimeInterpreter) GetEmails() []mail.Email {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	return interpreter.emails
}

func (interpreter *FakeLunchtimeInterpreter) GetTs() []time.Time {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	return interpreter.ts
}

func (interpreter *FakeLunchtimeInterpreter) GetPriorGameKeys() []string {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	return interpreter.priorGameKeys
}

func (interpreter *FakeLunchtimeInterpreter) GetMostRecentEmail() mail.Email {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	if len(interpreter.emails) == 0 {
		return mail.Email{}
	}

	return interpreter.emails[len(interpreter.emails)-1]
}

func (interpreter *FakeLunchtimeInterpreter) GetMostRecentT() time.Time {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	if len(interpreter.ts) == 0 {
		return time.Time{}
	}

	return interpreter.ts[len(interpreter.ts)-1]
}

func (interpreter *FakeLunchtimeInterpreter) GetMostRecentGameKeys() string {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	if len(interpreter.priorGameKeys) == 0 {
		return ""
	}

	return interpreter.priorGameKeys[len(interpreter.priorGameKeys)-1]
}

func (interpreter *FakeLunchtimeInterpreter) SetCommand(command Command) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.returnCommand = command
}

func (interpreter *FakeLunchtimeInterpreter) SetError(err error) {
	interpreter.lock.Lock()
	defer interpreter.lock.Unlock()

	interpreter.returnErr = err
}
