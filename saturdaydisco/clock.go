package saturdaydisco

import (
	"context"
	"time"
)

var Timezone *time.Location

func init() {
	var err error
	Timezone, err = time.LoadLocation("America/Denver")
	if err != nil {
		panic(err)
	}
}

func NextSaturdayAt10(now time.Time) time.Time {
	now = now.In(Timezone)
	if now.Weekday() == time.Saturday && now.Hour() >= 10 {
		return time.Date(now.Year(), now.Month(), now.Day()+7, 10, 0, 0, 0, Timezone)
	}
	deltaDay := int(time.Saturday - now.Weekday())
	return time.Date(now.Year(), now.Month(), now.Day()+deltaDay, 10, 0, 0, 0, Timezone)
}

type AlarmClockInt interface {
	Time() time.Time
	C() <-chan time.Time
	SetAlarm(time.Time)
	Stop()
}

type AlarmClock struct {
	c      chan time.Time
	cancel func()
}

func NewAlarmClock() *AlarmClock {
	c := make(chan time.Time)
	return &AlarmClock{
		c: c,
	}
}

func (a *AlarmClock) Time() time.Time {
	return time.Now()
}

func (a *AlarmClock) C() <-chan time.Time {
	return a.c
}

func (a *AlarmClock) SetAlarm(t time.Time) {
	a.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	go func() {
		timer := time.NewTimer(time.Until(t))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case t := <-timer.C:
			select {
			case a.c <- t:
			case <-ctx.Done():
			}
		}
	}()
}

func (a *AlarmClock) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}
