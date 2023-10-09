package lunchtimedisco

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"
	"time"

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

type LunchtimeDiscoState string

const (
	StateInvalid LunchtimeDiscoState = `invalid`

	StatePending LunchtimeDiscoState = `pending`

	//monitoring
	StateMonitoring LunchtimeDiscoState = `monitoring`

	//end states
	StateNoInviteSent LunchtimeDiscoState = `no_invite_sent`
	StateNoGameSent   LunchtimeDiscoState = `no_game_sent`
	StateGameOnSent   LunchtimeDiscoState = `game_on_sent`
	StateReminderSent LunchtimeDiscoState = `reminder_sent`
	StateAbort        LunchtimeDiscoState = `abort`
)

type CommandType string

const (
	CommandCaptureThreadEmail CommandType = "capture_thread_email"

	CommandAdminStatus CommandType = "admin_status"
	CommandAdminAbort  CommandType = "admin_abort"
	CommandAdminReset  CommandType = "admin_reset"
	CommandAdminDebug  CommandType = "admin_debug"

	CommandAdminBadger   CommandType = "admin_badger"
	CommandAdminGameOn   CommandType = "admin_game_on"
	CommandAdminNoGame   CommandType = "admin_no_game"
	CommandAdminInvite   CommandType = "admin_invite"
	CommandAdminNoInvite CommandType = "admin_no_invite"
	CommandAdminSetGames CommandType = "admin_set_games"
	CommandAdminInvalid  CommandType = "admin_invalid"

	CommandPlayerStatus      CommandType = "player_status"
	CommandPlayerUnsubscribe CommandType = "player_unsubscribe"
	CommandPlayerSetGames    CommandType = "player_set_games"
	CommandPlayerUnsure      CommandType = "player_unsure"
	CommandPlayerError       CommandType = "player_error"
)

type Command struct {
	CommandType       CommandType
	Email             mail.Email
	AdditionalContent string

	//for set games
	EmailAddress mail.EmailAddress
	GameKeyInput string

	//for game-on
	GameOnGameKey string

	Error error
}

type ProcessedEmailIDs []string

func (p ProcessedEmailIDs) Contains(id string) bool {
	for _, processedID := range p {
		if processedID == id {
			return true
		}
	}
	return false
}

type LunchtimeDiscoSnapshot struct {
	ThreadEmail       mail.Email            `json:"thread_email"`
	State             LunchtimeDiscoState   `json:"state"`
	Participants      LunchtimeParticipants `json:"participants"`
	NextEvent         time.Time             `json:"next_event"`
	T                 time.Time             `json:"reference_time"`
	ProcessedEmailIDs ProcessedEmailIDs     `json:"processed_email_ids"`
	GameOnGameKey     string
}

func (s LunchtimeDiscoSnapshot) dup() LunchtimeDiscoSnapshot {
	return LunchtimeDiscoSnapshot{
		ThreadEmail:   s.ThreadEmail.Dup(),
		State:         s.State,
		Participants:  s.Participants.dup(),
		NextEvent:     s.NextEvent,
		T:             s.T,
		GameOnGameKey: s.GameOnGameKey,
	}
}

type LunchtimeDisco struct {
	LunchtimeDiscoSnapshot
	w io.Writer

	alarmClock  clock.AlarmClockInt
	outbox      mail.OutboxInt
	interpreter LunchtimeInterpreterInt
	forecaster  weather.ForecasterInt
	db          s3db.S3DBInt
	commandC    chan Command
	snapshotC   chan chan<- LunchtimeDiscoSnapshot
	templateC   chan chan<- TemplateData
	config      config.Config
	ctx         context.Context
	cancel      func()
}

type TemplateData struct {
	NextEvent string
	LunchtimeDiscoSnapshot

	WeekOf     string
	Games      Games
	GameOnGame Game
	GameOff    bool

	Message       string
	Error         error
	Attachment    any
	EmailDebugKey string
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

func (e TemplateData) WithError(err error) TemplateData {
	e.Error = err
	return e
}

func (e TemplateData) WithAttachment(attachment any) TemplateData {
	e.Attachment = attachment
	return e
}

func (e TemplateData) WithEmailDebugKey(key string) TemplateData {
	e.EmailDebugKey = key
	return e
}

func NewLunchtimeDisco(config config.Config, w io.Writer, alarmClock clock.AlarmClockInt, outbox mail.OutboxInt, interpreter LunchtimeInterpreterInt, forecaster weather.ForecasterInt, db s3db.S3DBInt) (*LunchtimeDisco, error) {
	lunchtimeDisco := &LunchtimeDisco{
		alarmClock:  alarmClock,
		outbox:      outbox,
		interpreter: interpreter,
		forecaster:  forecaster,
		db:          db,
		commandC:    make(chan Command),
		snapshotC:   make(chan chan<- LunchtimeDiscoSnapshot),
		templateC:   make(chan chan<- TemplateData),
		w:           w,

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

	if err != nil {
		outbox.SendEmail(lunchtimeDisco.emailForBoss("startup_error", TemplateData{
			Error: fmt.Errorf(startupMessage),
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

// called asynchronously by the server
func (s *LunchtimeDisco) HandleIncomingEmail(email mail.Email) {
	go func() {
		s.processEmail(email)
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
		WeekOf:                 s.T.Add(-24 * 5).Format("1/2"),
		LunchtimeDiscoSnapshot: s.LunchtimeDiscoSnapshot,
		Games:                  games,
		GameOnGame:             gameOnGame,
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
	return mail.E().
		WithFrom(s.config.LunchtimeDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject(s.emailSubject(name, data)).
		WithBody(s.emailBody(name, data))
}

func (s *LunchtimeDisco) emailForList(name string, data TemplateData) mail.Email {
	if s.ThreadEmail.MessageID == "" {
		return mail.E().
			WithFrom(s.config.LunchtimeDiscoEmail).
			WithTo(s.config.LunchtimeDiscoList).
			WithSubject(s.emailSubject(name, data)).
			WithBody(mail.Markdown(s.emailBody(name, data)))
	} else {
		email := mail.E().
			WithFrom(s.config.LunchtimeDiscoEmail).
			WithTo(s.config.LunchtimeDiscoList).
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

var setCommandRegex = regexp.MustCompile(`^/set\s+(.+)+\s+(.*)$`)
var gameOnRegex = regexp.MustCompile(`^/game-on\s+([ABCDEFGHIJKLMNOP])$`)

func (s *LunchtimeDisco) processEmail(email mail.Email) {
	s.logi(0, "{{yellow}}Processing Email:{{/}}")
	s.logi(1, "Email: %s", email.String())
	c := Command{Email: email}
	isFromSelf := email.From.Equals(s.config.SaturdayDiscoEmail)
	isFromSelfToList := isFromSelf && email.IncludesRecipient(s.config.LunchtimeDiscoList)
	isAdminCommand := email.From.Equals(s.config.BossEmail) &&
		len(email.To) == 1 &&
		len(email.CC) == 0 &&
		email.To[0].Equals(s.config.LunchtimeDiscoEmail)
	isPlayerEmail := !email.From.Equals(s.config.BossEmail) && !isFromSelf

	if isAdminCommand {
		commandLine := strings.Split(strings.TrimSpace(email.Text), "\n")[0]
		if match := setCommandRegex.FindAllStringSubmatch(commandLine, -1); match != nil {
			c.CommandType = CommandAdminSetGames
			c.EmailAddress = mail.EmailAddress(match[0][1])
			c.GameKeyInput = match[0][2]
		} else if strings.HasPrefix(commandLine, "/status") {
			c.CommandType = CommandAdminStatus
		} else if strings.HasPrefix(commandLine, "/debug") {
			c.CommandType = CommandAdminDebug
		} else if strings.HasPrefix(commandLine, "/abort") {
			c.CommandType = CommandAdminAbort
		} else if strings.HasPrefix(email.Text, "/RESET-RESET-RESET") {
			c.CommandType = CommandAdminReset
		} else if match := gameOnRegex.FindAllStringSubmatch(commandLine, -1); match != nil {
			c.CommandType = CommandAdminGameOn
			c.GameOnGameKey = match[0][1]
		} else if strings.HasPrefix(commandLine, "/no-game") {
			c.CommandType = CommandAdminNoGame
		} else if strings.HasPrefix(commandLine, "/badger") {
			c.CommandType = CommandAdminNoGame
		} else if strings.HasPrefix(commandLine, "/invite") {
			c.CommandType = CommandAdminInvite
		} else if strings.HasPrefix(commandLine, "/no-invite") {
			c.CommandType = CommandAdminNoInvite
		} else {
			c.CommandType = CommandAdminInvalid
			c.Error = fmt.Errorf("invalid command: %s", commandLine)
		}
		switch c.CommandType {
		case CommandAdminGameOn, CommandAdminNoGame, CommandAdminBadger, CommandAdminInvite, CommandAdminNoInvite:
			idxFirstNewline := strings.Index(email.Text, "\n")
			if idxFirstNewline > -1 {
				c.AdditionalContent = strings.Trim(email.Text[idxFirstNewline:], "\n")
			}
		}
	} else if isPlayerEmail {
		potentialCommand, err := s.interpreter.InterpretEmail(email, s.T, s.Participants.GamesFor(email.From))
		if err != nil {
			c.CommandType = CommandPlayerError
			c.Error = err
		} else {
			c = potentialCommand
		}
	} else if isFromSelfToList {
		c.CommandType = CommandCaptureThreadEmail
	} else {
		return
	}

	if c.Error != nil {
		s.logi(1, "{{red}}unable to extract command from email: %s - %s{{/}}", c.CommandType, c.Error.Error())
	}

	s.commandC <- c
}

func (s *LunchtimeDisco) gameOnGameTime() time.Time {
	return s.T.Add(DT[s.GameOnGameKey])
}

func (s *LunchtimeDisco) transitionTo(state LunchtimeDiscoState) {
	switch state {
	case StatePending:
		s.NextEvent = s.T.Add(-6*day - 4*time.Hour) //Sunday, 6am
	case StateMonitoring:
		s.NextEvent = clock.DayOfAt6am(s.alarmClock.Time().Add(day)) //ping again the next morning
	case StateGameOnSent:
		s.NextEvent = clock.DayOfAt6am(s.gameOnGameTime()) //schedule reminder for morning of winning game
	case StateNoInviteSent, StateNoGameSent, StateReminderSent, StateAbort:
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
	case StatePending, StateMonitoring:
		s.logi(1, "{{coral}}sending boss the morning ping{{/}}")
		targetState := StateMonitoring
		if s.alarmClock.Time().Weekday() >= time.Thursday {
			targetState = StateAbort
		}
		s.sendEmail(s.emailForBoss("monitor", data), targetState, s.retryNextEventErrorHandler)
	case StateGameOnSent:
		s.sendEmail(s.emailForList("reminder", data), StateReminderSent, s.retryNextEventErrorHandler)
	case StateNoInviteSent, StateNoGameSent, StateReminderSent, StateAbort:
		s.reset()
	}
}

func (s *LunchtimeDisco) handleCommand(command Command) {
	if s.ProcessedEmailIDs.Contains(command.Email.MessageID) {
		s.logi(1, "{{coral}}I've already processed this email (id: %s).  Ignoring.{{/}}", command.Email.MessageID)
		return
	}
	defer func() {
		s.ProcessedEmailIDs = append(s.ProcessedEmailIDs, command.Email.MessageID)
	}()
	switch command.CommandType {
	case CommandCaptureThreadEmail:
		s.logi(1, "{{green}}capturing thread email{{/}}")
		s.ThreadEmail = command.Email
	case CommandAdminStatus:
		s.logi(1, "{{green}}boss is asking for status{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
			s.emailBody("boss_status", s.emailData())))
	case CommandAdminReset:
		s.logi(1, "{{red}}BOSS IS RESETTING THE SYSTEM.  HOLD ON TO YOUR BUTTS.{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
			s.emailBody("reset", s.emailData())))
		s.reset()
	case CommandAdminDebug:
		s.logi(1, "{{green}}boss is asking for debug info{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
			mail.Markdown(s.emailBody("boss_debug",
				s.emailData().
					WithMessage("Here's what a **multiline message** looks like.\n\n_Woohoo!_").
					WithError(fmt.Errorf("And this is what an error looks like!"))),
			)))
	case CommandAdminAbort:
		s.logi(1, "{{red}}boss has asked me to abort{{/}}")
		s.sendEmail(command.Email.Reply(s.config.LunchtimeDiscoEmail,
			s.emailBody("abort", s.emailData())),
			StateAbort, s.replyWithFailureErrorHandler)
	case CommandAdminBadger:
		s.logi(1, "{{red}}boss has asked me to badger{{/}}")
		s.sendEmailWithNoTransition((s.emailForList("badger",
			s.emailData().WithMessage(command.AdditionalContent))))
	case CommandAdminGameOn:
		s.logi(1, "{{green}}boss has asked me to send game-on{{/}}")
		s.sendEmail(s.emailForList("game_on",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateGameOnSent, s.replyWithFailureErrorHandler)
		s.GameOnGameKey = command.GameOnGameKey
	case CommandAdminNoGame:
		s.logi(1, "{{red}}boss has asked me to send no-game{{/}}")
		s.sendEmail(s.emailForList("no_game",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoGameSent, s.replyWithFailureErrorHandler)
	case CommandAdminInvite:
		s.logi(1, "{{green}}boss has asked me to send the invite out{{/}}")
		s.sendEmail(s.emailForList("invite",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateMonitoring, s.replyWithFailureErrorHandler)
	case CommandAdminNoInvite:
		s.logi(1, "{{red}}boss has asked me to send the no-invite email{{/}}")
		s.sendEmail(s.emailForList("no_invite",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoInviteSent, s.replyWithFailureErrorHandler)
	case CommandAdminSetGames:
		s.logi(1, "{{green}}boss has asked me to set games{{/}}")
		p, result, err := s.Participants.UpdateGameKeys(command.EmailAddress, command.GameKeyInput)
		if err != nil {
			s.logi(2, "{{red}}...failed: %s{{/}}", err.Error())
			s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
				s.emailBody("invalid_admin_email", s.emailData().WithError(err))))
		} else {
			s.logi(2, "{{green}}...success: %s{{/}}", result)
			s.Participants = p
			s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
				s.emailBody("acknowledge_admin_set_games",
					s.emailData().WithMessage("%s %s", command.EmailAddress, result))))
		}
	case CommandAdminInvalid:
		s.logi(1, "{{red}}boss sent me an invalid command{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.LunchtimeDiscoEmail,
			s.emailBody("invalid_admin_email",
				s.emailData().WithError(command.Error))))
	case CommandPlayerStatus:
		s.logi(1, "{{green}}player is asking for status.{{/}}")
		s.sendEmailWithNoTransition(command.Email.ReplyAll(s.config.SaturdayDiscoEmail,
			mail.Markdown(s.emailBody("public_status", s.emailData()))).AndCC(s.config.BossEmail))
	case CommandPlayerUnsubscribe:
		s.logi(1, "{{green}}player asking to unsubscribe.  Acking and looping in the boss.{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("unsubscribe_player_command", s.emailData())).AndCC(s.config.BossEmail))
	case CommandPlayerSetGames:
		s.logi(1, "{{green}}player has asked me to set games{{/}}")
		p, result, err := s.Participants.UpdateGameKeys(command.EmailAddress, command.GameKeyInput)
		if err != nil {
			s.logi(2, "{{red}}...failed: %s{{/}}", err.Error())
			s.sendEmailWithNoTransition(command.Email.Forward(s.config.LunchtimeDiscoEmail, s.config.BossEmail,
				s.emailBody("error_player_command", s.emailData().WithError(err))))
		} else {
			s.logi(2, "{{green}}...success: %s{{/}}", result)
			s.Participants = p
			s.sendEmailWithNoTransition(command.Email.Forward(s.config.LunchtimeDiscoEmail, s.config.BossEmail,
				s.emailBody("acknowledge_player_set_games",
					s.emailData().WithMessage("%s %s", command.EmailAddress, result).WithAttachment(command.EmailAddress).WithEmailDebugKey(command.Email.DebugKey))))
		}
	case CommandPlayerUnsure:
		s.logi(1, "{{red}}player sent a message that i'm unsure about.  CCing the boss and asking for help.{{/}}")
		s.sendEmailWithNoTransition(command.Email.Forward(s.config.SaturdayDiscoEmail, s.config.BossEmail,
			s.emailBody("unsure_player_command", s.emailData())))
	case CommandPlayerError:
		s.logi(1, "{{red}}encountered an error while processing a player command: %s{{/}}", command.Error.Error())
		s.sendEmailWithNoTransition(command.Email.Forward(s.config.SaturdayDiscoEmail, s.config.BossEmail,
			s.emailBody("error_player_command", s.emailData())))
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
	s.ThreadEmail = mail.Email{}
	s.Participants = LunchtimeParticipants{}
	s.NextEvent = time.Time{}
	s.T = clock.NextSaturdayAt10(s.alarmClock.Time())
	s.ProcessedEmailIDs = ProcessedEmailIDs{}
	s.GameOnGameKey = ""
	s.transitionTo(StatePending)
}
