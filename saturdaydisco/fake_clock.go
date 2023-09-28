package saturdaydisco

import (
	"sync"
	"time"
)

type FakeAlarmClock struct {
	now       time.Time
	alarmTime time.Time
	c         chan time.Time
	lock      *sync.Mutex
}

func NewFakeAlarmClock() *FakeAlarmClock {
	c := make(chan time.Time)
	return &FakeAlarmClock{
		c:    c,
		lock: &sync.Mutex{},
	}
}

func (a *FakeAlarmClock) SetTime(t time.Time) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.now = t
}

func (a *FakeAlarmClock) Fire() {
	a.lock.Lock()
	t := a.alarmTime
	a.lock.Unlock()
	a.now = t
	a.c <- t
}

// AlarmClockInt interface

func (a *FakeAlarmClock) Time() time.Time {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.now
}

func (a *FakeAlarmClock) C() <-chan time.Time {
	return a.c
}

func (a *FakeAlarmClock) SetAlarm(t time.Time) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.alarmTime = t
}

func (a *FakeAlarmClock) Stop() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.alarmTime = time.Time{}
}
