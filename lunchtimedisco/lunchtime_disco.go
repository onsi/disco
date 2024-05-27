package lunchtimedisco

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/disco/clock"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/weather"
	"github.com/onsi/say"
)

//go:embed templates
var templateFS embed.FS

var templates *template.Template

func init() {
	var err error
	templates, err = template.ParseFS(templateFS, "templates/*")
	if err != nil {
		panic(err)
	}
}

const day = 24 * time.Hour
const RETRY_DELAY = 5 * time.Minute

const KEY = "lunchtime-disco"
const PARTICIPANTS_KEY = "lunchtime-participants"

type LunchtimeDiscoState string

const (
	StateInvalid LunchtimeDiscoState = `invalid`

	StatePending    LunchtimeDiscoState = `pending`
	StateInviteSent LunchtimeDiscoState = `invite_sent`

	//end states
	StateNoInviteSent LunchtimeDiscoState = `no_invite_sent`
	StateNoGameSent   LunchtimeDiscoState = `no_game_sent`
	StateGameOnSent   LunchtimeDiscoState = `game_on_sent`
	StateReminderSent LunchtimeDiscoState = `reminder_sent`
)

type CommandType string

const (
	CommandCaptureThreadEmail CommandType = "capture_thread_email"

	CommandAdminBadger   CommandType = "admin_badger"
	CommandAdminGameOn   CommandType = "admin_game_on"
	CommandAdminNoGame   CommandType = "admin_no_game"
	CommandAdminInvite   CommandType = "admin_invite"
	CommandAdminNoInvite CommandType = "admin_no_invite"

	CommandSetGames CommandType = "set_games"
)

type Command struct {
	CommandType       CommandType `json:"commandType"`
	AdditionalContent string      `json:"additionalContent"`

	//for set games
	Participant LunchtimeParticipant `json:"participant"`

	//for game-on
	GameOnGameKey      string `json:"gameOnGameKey"`
	GameOnAdjustedTime string `json:"gameOnAdjustedTime"`

	Email mail.Email
	Error error
}

type LunchtimeDiscoSnapshot struct {
	GUID               string                `json:"guid"`
	BossGUID           string                `json:"boss_guid"`
	ThreadEmail        mail.Email            `json:"thread_email"`
	State              LunchtimeDiscoState   `json:"state"`
	Participants       LunchtimeParticipants `json:"participants"`
	NextEvent          time.Time             `json:"next_event"`
	T                  time.Time             `json:"reference_time"`
	GameOnGameKey      string                `json:"game_on_game_key"`
	GameOnAdjustedTime string                `json:"game_on_adjusted_time"`
}

func (s LunchtimeDiscoSnapshot) dup() LunchtimeDiscoSnapshot {
	return LunchtimeDiscoSnapshot{
		BossGUID:           s.BossGUID,
		GUID:               s.GUID,
		ThreadEmail:        s.ThreadEmail.Dup(),
		State:              s.State,
		Participants:       s.Participants.dup(),
		NextEvent:          s.NextEvent,
		T:                  s.T,
		GameOnGameKey:      s.GameOnGameKey,
		GameOnAdjustedTime: s.GameOnAdjustedTime,
	}
}

type LunchtimeDisco struct {
	LunchtimeDiscoSnapshot
	HistoricalParticipants HistoricalParticipants
	w                      io.Writer

	alarmClock clock.AlarmClockInt
	outbox     mail.OutboxInt
	forecaster weather.ForecasterInt
	db         s3db.S3DBInt
	commandC   chan Command
	snapshotC  chan chan<- LunchtimeDiscoSnapshot
	templateC  chan chan<- TemplateData
	config     config.Config
	ctx        context.Context
	cancel     func()
}

type TemplateData struct {
	LunchtimeDiscoSnapshot
	HistoricalParticipants HistoricalParticipants
	NextEvent              string

	GUID               string
	BossGUID           string
	WeekOf             string
	Games              Games
	GameOnGame         Game
	GameOnAdjustedTime string
	GameOff            bool

	Message string
	Comment string
	Error   error
}

func (e TemplateData) GameOnGameFullStartTime() string {
	if e.GameOnAdjustedTime != "" {
		return e.GameOnGame.FullStartTimeWithAdjustedTime(e.GameOnAdjustedTime)
	} else {
		return e.GameOnGame.FullStartTime()
	}
}

func (e TemplateData) GameOnGameStartTime() string {
	if e.GameOnAdjustedTime != "" {
		return e.GameOnAdjustedTime
	} else {
		return e.GameOnGame.GameTime()
	}
}

func (e TemplateData) WithNextEvent(t time.Time) TemplateData {
	e.NextEvent = t.In(clock.Timezone).Format("Monday 1/2 3:04pm")
	return e
}

func (e TemplateData) WithMessage(format string, args ...any) TemplateData {
	if len(args) == 0 {
		e.Message = format
	} else {
		e.Message = fmt.Sprintf(format, args...)
	}
	return e
}

func (e TemplateData) WithComment(comment string) TemplateData {
	e.Comment = comment
	return e
}

func (e TemplateData) WithError(err error) TemplateData {
	e.Error = err
	return e
}

func (e TemplateData) PickerURL() string {
	return fmt.Sprintf("https://www.sedenverultimate.net/lunchtime/%s", e.GUID)
}

func (e TemplateData) BossURL() string {
	return fmt.Sprintf("https://www.sedenverultimate.net/lunchtime/%s", e.BossGUID)
}

func (e TemplateData) JSONForPlayer() string {
	games := map[string]map[string]any{}
	for _, game := range e.Games {
		games[game.Key] = map[string]any{
			"key":      game.Key,
			"date":     game.GameDate(),
			"time":     game.GameTime(),
			"forecast": game.Forecast,
		}
	}

	out, _ := json.Marshal(map[string]any{
		"guid":                    e.GUID,
		"weekOf":                  e.WeekOf,
		"participants":            e.Participants,
		"games":                   games,
		"gameOnGameKey":           e.GameOnGameKey,
		"gameOnGameFullStartTime": e.GameOnGameFullStartTime(),
	})
	return string(out)
}

func (e TemplateData) JSONForBoss() string {
	games := []map[string]any{}
	for _, game := range e.Games {
		games = append(games, map[string]any{
			"key":      game.Key,
			"day":      game.GameDay(),
			"date":     game.GameDate(),
			"time":     game.GameTime(),
			"forecast": game.Forecast,
		})
	}

	out, _ := json.Marshal(map[string]any{
		"state":                   e.State,
		"bossGuid":                e.BossGUID,
		"weekOf":                  e.WeekOf,
		"historicalParticipants":  e.HistoricalParticipants,
		"participants":            e.Participants,
		"games":                   games,
		"gameOnGameKey":           e.GameOnGameKey,
		"gameOnAdjustedTime":      e.GameOnAdjustedTime,
		"gameOnGameFullStartTime": e.GameOnGameFullStartTime(),
	})
	return string(out)
}

func NewLunchtimeDisco(config config.Config, w io.Writer, alarmClock clock.AlarmClockInt, outbox mail.OutboxInt, forecaster weather.ForecasterInt, db s3db.S3DBInt) (*LunchtimeDisco, error) {
	lunchtimeDisco := &LunchtimeDisco{
		alarmClock: alarmClock,
		outbox:     outbox,
		forecaster: forecaster,
		db:         db,
		commandC:   make(chan Command),
		snapshotC:  make(chan chan<- LunchtimeDiscoSnapshot),
		templateC:  make(chan chan<- TemplateData),
		w:          w,

		config: config,
	}
	lunchtimeDisco.ctx, lunchtimeDisco.cancel = context.WithCancel(context.Background())

	startupMessage := ""
	lastBackup, err := db.FetchObject(KEY)
	if err == s3db.ErrObjectNotFound {
		startupMessage = "No backup found, starting from scratch..."
		lunchtimeDisco.logi(0, "{{yellow}}%s{{/}}", startupMessage)
		lunchtimeDisco.reset()
		err = nil
	} else if err != nil {
		startupMessage = fmt.Sprintf("FAILED TO LOAD BACKUP: %s", err.Error())
		lunchtimeDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
	} else {
		lunchtimeDisco.logi(0, "{{green}}Loading from Backup...{{/}}")
		snapshot := LunchtimeDiscoSnapshot{}
		err = json.Unmarshal(lastBackup, &snapshot)
		if err != nil {
			startupMessage = fmt.Sprintf("FAILED TO UNMARSHAL BACKUP: %s", err.Error())
			lunchtimeDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
		} else {
			nextSaturday := clock.NextSaturdayAt10(alarmClock.Time())
			if nextSaturday.After(snapshot.T) {
				startupMessage = "Backup is from a previous week.  Resetting."
				lunchtimeDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
				lunchtimeDisco.reset()
			} else {
				startupMessage = "Backup is good.  Spinning up..."
				lunchtimeDisco.logi(0, "{{green}}%s{{/}}", startupMessage)
				lunchtimeDisco.LunchtimeDiscoSnapshot = snapshot
				alarmClock.SetAlarm(snapshot.NextEvent)
			}
		}
	}

	participantsMessage := ""
	historicalParticipants, pErr := db.FetchObject(PARTICIPANTS_KEY)
	if pErr == s3db.ErrObjectNotFound {
		participantsMessage = "No historical participants found.  Starting from scratch..."
		lunchtimeDisco.logi(0, "{{yellow}}%s{{/}}", participantsMessage)
		pErr = nil
	} else if pErr != nil {
		participantsMessage = fmt.Sprintf("FAILED TO LOAD HISTORICAL PARTICIPANTS: %s", err.Error())
		lunchtimeDisco.logi(0, "{{red}}%s{{/}}", participantsMessage)
	} else {
		lunchtimeDisco.logi(0, "{{green}}Loading historical participants...{{/}}")
		pErr = json.Unmarshal(historicalParticipants, &(lunchtimeDisco.HistoricalParticipants))
		if pErr != nil {
			participantsMessage = fmt.Sprintf("FAILED TO UNMARSHAL HISTORICAL PARTICIPANTS: %s", err.Error())
			lunchtimeDisco.logi(0, "{{red}}%s{{/}}", participantsMessage)
		} else {
			participantsMessage = "Historical participants loaded."
			lunchtimeDisco.logi(0, "{{green}}%s{{/}}", participantsMessage)
		}
	}

	if err != nil {
		outbox.SendEmail(lunchtimeDisco.emailForBoss("startup_error", TemplateData{
			Error: fmt.Errorf(startupMessage + "\n" + participantsMessage),
		}))
		return nil, err
	}

	outbox.SendEmail(lunchtimeDisco.emailForBoss("startup", lunchtimeDisco.emailData().WithMessage(startupMessage)))

	go lunchtimeDisco.dance()
	return lunchtimeDisco, nil
}

func (s *LunchtimeDisco) Stop() {
	s.cancel()
	s.alarmClock.Stop()
}

func (s *LunchtimeDisco) HandleIncomingEmail(email mail.Email) {
	go func() {
		s.processEmail(email)
	}()
}

// submission from a user
func (s *LunchtimeDisco) HandleParticipant(participant LunchtimeParticipant) {
	go func() {
		s.commandC <- Command{
			CommandType: CommandSetGames,
			Participant: participant,
		}
	}()
}

// command from the Boss
func (s *LunchtimeDisco) HandleCommand(command Command) {
	go func() {
		s.commandC <- command
	}()
}

func (s *LunchtimeDisco) GetSnapshot() LunchtimeDiscoSnapshot {
	c := make(chan LunchtimeDiscoSnapshot)
	s.snapshotC <- c
	return <-c
}

func (s *LunchtimeDisco) TemplateData() TemplateData {
	c := make(chan TemplateData)
	s.templateC <- c
	return <-c
}

func (s *LunchtimeDisco) log(format string, args ...any) {
	s.logi(0, format, args...)
}

func (s *LunchtimeDisco) logi(i uint, format string, args ...any) {
	out := say.F("{{gray}}[%s]{{/}} LunchtimeDisco: ", s.alarmClock.Time().Format("1/2 3:04:05am"))
	out += say.Fi(i, format, args...) + "\n"
	s.w.Write([]byte(out))
}

func (s *LunchtimeDisco) emailData() TemplateData {
	games := BuildGames(s.w, s.T, s.Participants, s.forecaster)
	var gameOnGame Game
	if s.GameOnGameKey != "" {
		gameOnGame = games.Game(s.GameOnGameKey)
	}
	return TemplateData{
		GUID:                   s.GUID,
		BossGUID:               s.BossGUID,
		WeekOf:                 s.T.Add(-day * 5).Format("1/2"),
		LunchtimeDiscoSnapshot: s.LunchtimeDiscoSnapshot,
		Games:                  games,
		GameOnGame:             gameOnGame,
		GameOnAdjustedTime:     s.GameOnAdjustedTime,
		HistoricalParticipants: s.HistoricalParticipants,
		GameOff:                s.State == StateNoInviteSent || s.State == StateNoGameSent,
	}.WithNextEvent(s.NextEvent)
}

func (s *LunchtimeDisco) emailSubject(name string, data TemplateData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_subject", data)
	return b.String()
}

func (s *LunchtimeDisco) emailBody(name string, data TemplateData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_body", data)
	return b.String()
}

func (s *LunchtimeDisco) emailForBoss(name string, data TemplateData) mail.Email {
	e := mail.E().
		WithFrom(s.config.LunchtimeDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject(s.emailSubject(name, data)).
		WithBody(mail.Markdown(s.emailBody(name, data)))
	return e
}

func (s *LunchtimeDisco) emailForList(name string, data TemplateData) mail.Email {
	if s.ThreadEmail.MessageID == "" {
		return mail.E().
			WithFrom(s.config.BossEmail).
			WithTo(s.config.LunchtimeDiscoList).
			AndCC(s.config.BossEmail).
			WithSubject(s.emailSubject(name, data)).
			WithBody(mail.Markdown(s.emailBody(name, data)))
	} else {
		email := mail.E().
			WithFrom(s.config.BossEmail).
			WithTo(s.config.LunchtimeDiscoList).
			AndCC(s.config.BossEmail).
			WithBody(mail.Markdown(s.emailBody(name, data)))
		if strings.HasPrefix(s.ThreadEmail.Subject, "Re: ") {
			email.Subject = s.ThreadEmail.Subject
		} else {
			email.Subject = "Re: " + s.ThreadEmail.Subject
		}
		email.InReplyTo = s.ThreadEmail.MessageID
		return email
	}
}

func (s *LunchtimeDisco) dance() {
	s.log("{{green}}on the dance floor{{/}}")
	for {
		select {
		case <-s.ctx.Done():
			s.log("{{green}}leaving the dance floor{{/}}")
			return
		case <-s.alarmClock.C():
			s.log("{{yellow}}alarm clock triggered{{/}}")
			s.performNextEvent()
			s.backup()
		case command := <-s.commandC:
			s.log("{{yellow}}received a command{{/}}")
			s.handleCommand(command)
			s.backup()
			if command.CommandType == CommandSetGames {
				s.storeHistoricalParticipants()
			}
		case c := <-s.templateC:
			c <- s.emailData()
		case c := <-s.snapshotC:
			c <- s.LunchtimeDiscoSnapshot.dup()
		}
	}
}

func (s *LunchtimeDisco) backup() {
	s.log("{{yellow}}backing up...{{/}}")
	data, err := json.Marshal(s.LunchtimeDiscoSnapshot)
	if err != nil {
		s.log("{{red}}failed to marshal backup: %s{{/}}", err.Error())
		return
	}
	err = s.db.PutObject(KEY, data)
	if err != nil {
		s.log("{{red}}failed to backup: %s{{/}}", err.Error())
		return
	}
	s.log("{{green}}backed up{{/}}")
}

func (s *LunchtimeDisco) storeHistoricalParticipants() {
	s.log("{{yellow}}storing historical participants...{{/}}")
	data, err := json.Marshal(s.HistoricalParticipants)
	if err != nil {
		s.log("{{red}}failed to marshal historical participants: %s{{/}}", err.Error())
		return
	}
	err = s.db.PutObject(PARTICIPANTS_KEY, data)
	if err != nil {
		s.log("{{red}}failed to store historical participants: %s{{/}}", err.Error())
		return
	}
	s.log("{{green}}stored historical participants{{/}}")
}

func (s *LunchtimeDisco) processEmail(email mail.Email) {
	s.logi(0, "{{yellow}}Processing Email:{{/}}")
	if email.From.Equals(s.config.BossEmail) && email.IncludesRecipient(s.config.LunchtimeDiscoList) {
		s.logi(1, "{{green}}This is a list email - harvesting the thread id{{/}}")
		s.commandC <- Command{
			CommandType: CommandCaptureThreadEmail,
			Email:       email,
		}
	} else {
		s.logi(1, "{{yellow}}Nothing to see here... move along.{{/}}")
	}
}

func (s *LunchtimeDisco) transitionTo(state LunchtimeDiscoState) {
	switch state {
	case StatePending, StateInviteSent:
		if s.NextEvent.IsZero() {
			s.NextEvent = clock.DayOfAt6am(s.T.Add(day * -6)) //start pinging on Sunday morning
		} else {
			s.NextEvent = clock.DayOfAt6am(s.alarmClock.Time().Add(day)) //ping again the next morning
		}
	case StateGameOnSent:
		s.NextEvent = clock.DayOfAt6am(s.T.Add(DT[s.GameOnGameKey])) //schedule reminder for morning of winning game
	case StateNoInviteSent, StateNoGameSent, StateReminderSent:
		s.NextEvent = s.T.Add(2 * time.Hour) //Saturday, 12pm is when we reset
	}
	s.State = state
	if !s.NextEvent.IsZero() {
		s.alarmClock.SetAlarm(s.NextEvent)
	}
}

func (s *LunchtimeDisco) performNextEvent() {
	data := s.emailData()
	switch s.State {
	case StatePending, StateInviteSent:
		s.logi(1, "{{coral}}sending boss the morning ping{{/}}")
		s.sendEmail(s.emailForBoss("monitor", data), s.State, s.retryNextEventErrorHandler)
	case StateGameOnSent:
		s.sendEmail(s.emailForList("reminder", data), StateReminderSent, s.retryNextEventErrorHandler)
	case StateNoInviteSent, StateNoGameSent, StateReminderSent:
		s.reset()
	}
}

func (s *LunchtimeDisco) handleCommand(command Command) {
	switch command.CommandType {
	case CommandCaptureThreadEmail:
		if s.ThreadEmail.IsZero() {
			s.logi(1, "{{green}}capturing thread email{{/}}")
			s.ThreadEmail = command.Email
		}
	case CommandAdminBadger:
		s.logi(1, "{{red}}boss has asked me to badger{{/}}")
		s.sendEmailWithNoTransition((s.emailForList("badger",
			s.emailData().WithMessage(command.AdditionalContent))))
	case CommandAdminGameOn:
		s.logi(1, "{{green}}boss has asked me to send game-on{{/}}")
		s.GameOnGameKey = command.GameOnGameKey
		s.GameOnAdjustedTime = command.GameOnAdjustedTime
		s.sendEmail(s.emailForList("game_on",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateGameOnSent, s.replyWithFailureErrorHandler)
	case CommandAdminNoGame:
		s.logi(1, "{{red}}boss has asked me to send no-game{{/}}")
		s.GameOnGameKey = ""
		s.GameOnAdjustedTime = ""
		s.sendEmail(s.emailForList("no_game",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoGameSent, s.replyWithFailureErrorHandler)
	case CommandAdminInvite:
		s.logi(1, "{{green}}boss has asked me to send the invite out{{/}}")
		s.sendEmail(s.emailForList("invitation",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateInviteSent, s.replyWithFailureErrorHandler)
	case CommandAdminNoInvite:
		s.logi(1, "{{red}}boss has asked me to send the no-invite email{{/}}")
		s.sendEmail(s.emailForList("no_invitation",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoInviteSent, s.replyWithFailureErrorHandler)
	case CommandSetGames:
		s.logi(1, "{{green}}I've been asked to set games{{/}}")
		s.Participants = s.Participants.AddOrUpdate(command.Participant)
		s.HistoricalParticipants = s.HistoricalParticipants.AddOrUpdate(command.Participant.Address)
		s.sendEmailWithNoTransition(s.emailForBoss("acknowledge_set_games", s.emailData().
			WithMessage(command.Participant.GamesAckMessage()).
			WithComment(command.Participant.Comments)))
	}
}

func (s *LunchtimeDisco) retryNextEventErrorHandler(email mail.Email, err error) {
	s.outbox.SendEmail(mail.Email{
		From:    s.config.LunchtimeDiscoEmail,
		To:      []mail.EmailAddress{s.config.BossEmail},
		Subject: "Help!",
		Text:    fmt.Sprintf("Saturday Disco failed to send an e-mail during an event transition.\n\n%s\n\nTrying to send:\n\n%s\n\nPlease help!", err.Error(), email.String()),
	})
	s.alarmClock.SetAlarm(s.alarmClock.Time().Add(RETRY_DELAY))
}

func (s *LunchtimeDisco) replyWithFailureErrorHandler(email mail.Email, err error) {
	s.logi(1, "{{red}}failed while handling a command: %s{{/}}", err.Error())
	s.outbox.SendEmail(mail.Email{
		From:    s.config.LunchtimeDiscoEmail,
		To:      []mail.EmailAddress{s.config.BossEmail},
		Subject: "Help!",
		Text:    fmt.Sprintf("Saturday Disco failed while trying to handle a command.\n\n%s\n\nTrying to send:\n\n%s\n\nPlease help!", err.Error(), email.String()),
	})
}

func (s *LunchtimeDisco) sendEmail(email mail.Email, successState LunchtimeDiscoState, onFailure func(mail.Email, error)) {
	err := s.outbox.SendEmail(email)
	if err != nil {
		s.logi(1, "{{red}}failed to send e-mail: %s{{/}}", err.Error())
		s.logi(2, "current state: %s", s.State)
		s.logi(2, "target state: %s", successState)
		s.logi(2, "email: %s", email)

		onFailure(email, err)
	} else {
		s.transitionTo(successState)
	}
}

func (s *LunchtimeDisco) sendEmailWithNoTransition(email mail.Email) {
	err := s.outbox.SendEmail(email)
	if err != nil {
		s.logi(1, "{{red}}failed to send e-mail: %s{{/}}", err.Error())
		s.logi(2, "email: %s", email)
	}
}

func (s *LunchtimeDisco) reset() {
	s.alarmClock.Stop()
	s.State = StateInvalid
	s.GUID = uuid.New().String()
	s.BossGUID = uuid.New().String()
	s.ThreadEmail = mail.Email{}
	s.Participants = LunchtimeParticipants{}
	s.NextEvent = time.Time{}
	s.T = clock.NextSaturdayAt10(s.alarmClock.Time())
	s.GameOnGameKey = ""
	s.GameOnAdjustedTime = ""
	s.transitionTo(StatePending)
}
