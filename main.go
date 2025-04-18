package main

import (
	"encoding/json"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/saturdaydisco"
	"github.com/onsi/disco/server"
	"github.com/onsi/disco/weather"
	"github.com/onsi/say"
)

func main() {
	conf := config.LoadConfig()
	e := echo.New()
	var forecaster *weather.Forecaster
	var outbox mail.OutboxInt
	var db s3db.S3DBInt
	var saturdayDisco *saturdaydisco.SaturdayDisco
	var lunchtimeDisco *lunchtimedisco.LunchtimeDisco
	var err error

	if conf.IsDev() {
		db = s3db.NewFakeS3DB()
		realDb, err := s3db.NewS3DB()
		say.ExitIfError("could not build S3 DB", err)
		fakeOutbox := mail.NewFakeOutbox()
		fakeOutbox.EnableLogging(e.Logger.Output())
		outbox = fakeOutbox
		forecaster = weather.NewForecaster(realDb) //let's actually cache the emoji!

		// some fake data just so we can better inspect the web page
		blob, _ := json.Marshal(saturdaydisco.SaturdayDiscoSnapshot{
			State: saturdaydisco.StateGameOnSent,
			Participants: saturdaydisco.Participants{
				{Address: "Onsi Fakhouri <onsijoe@gmail.com>", Count: 1},
				{Address: "Jane Player <jane@example.com>", Count: 2},
				{Address: "Josh Player <josh@example.com>", Count: 1},
				{Address: "Nope Player <nope@example.com>", Count: 0},
				{Address: "Team Player <team@example.com>", Count: 3},
				{Address: "sally@example.com", Count: 1},
			},
			NextEvent: time.Now().Add(24 * time.Hour * 10),
			T:         clock.NextSaturdayAt10(time.Now()),
		})
		db.PutObject(saturdaydisco.KEY, blob)

		// some fake data just so we can better inspect the web page
		blob, _ = json.Marshal(lunchtimedisco.LunchtimeDiscoSnapshot{
			BossGUID: "boss",
			GUID:     "dev",
			State:    lunchtimedisco.StatePending,
			Participants: lunchtimedisco.LunchtimeParticipants{
				{Address: "Onsi Fakhouri <onsijoe@gmail.com>", GameKeys: []string{"A", "E", "F", "G", "I", "L", "M", "N"}},
				{Address: "Jane Player <jane@example.com>", GameKeys: []string{"A"}},
				{Address: "Josh Player <josh@example.com>", GameKeys: []string{"A", "B", "C"}},
				{Address: "Nope Player <nope@example.com>", GameKeys: []string{"A", "B", "D"}},
				{Address: "Team Player <team@example.com>", GameKeys: []string{"A", "B", "C"}},
				{Address: "Sally <sally@example.com>", GameKeys: []string{"E"}},
				{Address: "jude@example.com", GameKeys: []string{"E"}},
			},
			NextEvent: time.Now().Add(24 * time.Hour * 10),
			T:         clock.NextSaturdayAt10(time.Now()),
		})
		db.PutObject(lunchtimedisco.KEY, blob)

		blob, _ = json.Marshal(lunchtimedisco.HistoricalParticipants{
			"Alice Player <alice@example.com>",
			"Eric Player <eric@example.com>",
			"Onsi Fakhouri <onsijoe@gmail.com>",
			"Jane Player <jane@example.com>",
			"Josh Player <josh@example.com>",
			"Nope Player <nope@example.com>",
			"Sally <sally@example.com>",
			"jude@example.com",
		})
		db.PutObject(lunchtimedisco.PARTICIPANTS_KEY, blob)
	} else {
		db, err = s3db.NewS3DB()
		say.ExitIfError("could not build S3 DB", err)
		outbox = mail.NewOutbox(conf.ForwardEmailKey, conf.GmailUser, conf.GmailPassword)
		forecaster = weather.NewForecaster(db)
	}

	saturdayDisco, err = saturdaydisco.NewSaturdayDisco(
		conf,
		e.Logger.Output(),
		clock.NewAlarmClock(),
		outbox,
		saturdaydisco.NewInterpreter(e.Logger.Output()),
		forecaster,
		db,
	)
	say.ExitIfError("could not build Saturday Disco", err)

	lunchtimeDisco, err = lunchtimedisco.NewLunchtimeDisco(
		conf,
		e.Logger.Output(),
		clock.NewAlarmClock(),
		outbox,
		forecaster,
		db,
	)
	say.ExitIfError("could not build Lunchtime Disco", err)

	log.Fatal(server.NewServer(e, "./", conf, outbox, db, saturdayDisco, lunchtimeDisco).Start())
}
