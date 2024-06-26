package saturdaydisco

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
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

const QUORUM = 8

const day = 24 * time.Hour
const RETRY_DELAY = 5 * time.Minute
const ApprovalTime = 4 * time.Hour

const KEY = "saturday-disco"

type SaturdayDiscoState string

const (
	StateInvalid SaturdayDiscoState = `invalid`

	StatePending SaturdayDiscoState = `pending`

	//invitation
	StateRequestedInviteApproval SaturdayDiscoState = `requested_invite_approval`
	StateInviteSent              SaturdayDiscoState = `invite_sent`

	//badger
	StateRequestedBadgerApproval SaturdayDiscoState = `requested_badger_approval`
	StateBadgerSent              SaturdayDiscoState = `badger_sent`
	StateBadgerNotSent           SaturdayDiscoState = `badger_not_sent`

	//gameon/nogame
	StateRequestedGameOnApproval SaturdayDiscoState = `requested_game_on_approval`
	StateRequestedNoGameApproval SaturdayDiscoState = `requested_no_game_approval`

	//end states
	StateNoInviteSent SaturdayDiscoState = `no_invite_sent`
	StateNoGameSent   SaturdayDiscoState = `no_game_sent`
	StateGameOnSent   SaturdayDiscoState = `game_on_sent`
	StateReminderSent SaturdayDiscoState = `reminder_sent`
	StateAbort        SaturdayDiscoState = `abort`
)

type CommandType string

const (
	CommandRequestedInviteApprovalReply CommandType = `requested_invite_approval_reply`
	CommandRequestedBadgerApprovalReply CommandType = `requested_badger_approval_reply`
	CommandRequestedGameOnApprovalReply CommandType = "requested_game_on_approval_reply"
	CommandRequestedNoGameApprovalReply CommandType = "requested_no_game_approval_reply"
	CommandInvalidReply                 CommandType = "invalid_reply"

	CommandAdminStatus   CommandType = "admin_status"
	CommandAdminAbort    CommandType = "admin_abort"
	CommandAdminReset    CommandType = "admin_reset"
	CommandAdminGameOn   CommandType = "admin_game_on"
	CommandAdminNoGame   CommandType = "admin_no_game"
	CommandAdminSetCount CommandType = "admin_set_count"
	CommandAdminDebug    CommandType = "admin_debug"
	CommandAdminInvalid  CommandType = "admin_invalid"

	CommandPlayerSetCount CommandType = "player_set_count"
	CommandPlayerIgnore   CommandType = "player_ignore"
	CommandPlayerError    CommandType = "player_error"
)

type Command struct {
	CommandType CommandType
	Email       mail.Email

	Approved          bool
	Delay             int
	AdditionalContent string

	EmailAddress mail.EmailAddress
	Count        int

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

type SaturdayDiscoSnapshot struct {
	State             SaturdayDiscoState `json:"state"`
	Participants      Participants       `json:"participants"`
	NextEvent         time.Time          `json:"next_event"`
	T                 time.Time          `json:"reference_time"`
	ProcessedEmailIDs ProcessedEmailIDs  `json:"processed_email_ids"`
}

func (s SaturdayDiscoSnapshot) dup() SaturdayDiscoSnapshot {
	return SaturdayDiscoSnapshot{
		State:        s.State,
		Participants: s.Participants.dup(),
		NextEvent:    s.NextEvent,
		T:            s.T,
	}
}

type SaturdayDisco struct {
	SaturdayDiscoSnapshot
	w io.Writer

	alarmClock  clock.AlarmClockInt
	outbox      mail.OutboxInt
	interpreter InterpreterInt
	forecaster  weather.ForecasterInt
	db          s3db.S3DBInt
	commandC    chan Command
	snapshotC   chan chan<- SaturdayDiscoSnapshot
	templateC   chan chan<- TemplateData
	config      config.Config
	ctx         context.Context
	cancel      func()
}

type TemplateData struct {
	GameDate  string
	GameTime  string
	NextEvent string
	SaturdayDiscoSnapshot
	HasQuorum         bool
	GameOn            bool
	GameOff           bool
	Forecast          weather.Forecast
	DiscoEmailAddress string

	Message       string
	Error         error
	EmailDebugKey string
	Attachment    any
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

func NewSaturdayDisco(config config.Config, w io.Writer, alarmClock clock.AlarmClockInt, outbox mail.OutboxInt, interpreter InterpreterInt, forecaster weather.ForecasterInt, db s3db.S3DBInt) (*SaturdayDisco, error) {
	saturdayDisco := &SaturdayDisco{
		alarmClock:  alarmClock,
		outbox:      outbox,
		interpreter: interpreter,
		forecaster:  forecaster,
		db:          db,
		commandC:    make(chan Command),
		snapshotC:   make(chan chan<- SaturdayDiscoSnapshot),
		templateC:   make(chan chan<- TemplateData),
		w:           w,

		config: config,
	}
	saturdayDisco.ctx, saturdayDisco.cancel = context.WithCancel(context.Background())

	startupMessage := ""
	lastBackup, err := db.FetchObject(KEY)
	if err == s3db.ErrObjectNotFound {
		startupMessage = "No backup found, starting from scratch..."
		saturdayDisco.logi(0, "{{yellow}}%s{{/}}", startupMessage)
		saturdayDisco.reset()
		err = nil
	} else if err != nil {
		startupMessage = fmt.Sprintf("FAILED TO LOAD BACKUP: %s", err.Error())
		saturdayDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
	} else {
		saturdayDisco.logi(0, "{{green}}Loading from Backup...{{/}}")
		snapshot := SaturdayDiscoSnapshot{}
		err = json.Unmarshal(lastBackup, &snapshot)
		if err != nil {
			startupMessage = fmt.Sprintf("FAILED TO UNMARSHAL BACKUP: %s", err.Error())
			saturdayDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
		} else {
			nextSaturday := clock.NextSaturdayAt10Or1030(alarmClock.Time())
			if nextSaturday.After(snapshot.T) {
				startupMessage = "Backup is from a previous week.  Resetting."
				saturdayDisco.logi(0, "{{red}}%s{{/}}", startupMessage)
				saturdayDisco.reset()
			} else {
				startupMessage = "Backup is good.  Spinning up..."
				saturdayDisco.logi(0, "{{green}}%s{{/}}", startupMessage)
				saturdayDisco.SaturdayDiscoSnapshot = snapshot
				alarmClock.SetAlarm(snapshot.NextEvent)
			}
		}
	}

	if err != nil {
		outbox.SendEmail(saturdayDisco.emailForBoss("startup_error", TemplateData{
			Error: fmt.Errorf(startupMessage),
		}))
		return nil, err
	}

	if !alarmClock.Time().Before(saturdayDisco.T.Add(-2*day + 4*time.Hour)) {
		// it's after thursday at 2pm.  we had better already send the invite
		if saturdayDisco.State == StatePending || saturdayDisco.State == StateRequestedInviteApproval {
			//welp! we haven't sent it yet.
			saturdayDisco.logi(0, "{{red}}It's after Thursday at 2pm and we haven't sent the invite yet.  Aborting.{{/}}")
			startupMessage += "\nIt's after Thursday at 2pm and we haven't sent the invite yet.  Aborting.  You'll need to take over, boss."
			saturdayDisco.transitionTo(StateAbort)
		}
	}

	outbox.SendEmail(saturdayDisco.emailForBoss("startup", saturdayDisco.emailData().WithMessage(startupMessage)))

	go saturdayDisco.dance()
	return saturdayDisco, nil
}

func (s *SaturdayDisco) Stop() {
	s.cancel()
	s.alarmClock.Stop()
}

// called asynchronously by the server
func (s *SaturdayDisco) HandleIncomingEmail(email mail.Email) {
	go func() {
		s.processEmail(email)
	}()
}

func (s *SaturdayDisco) GetSnapshot() SaturdayDiscoSnapshot {
	c := make(chan SaturdayDiscoSnapshot)
	s.snapshotC <- c
	return <-c
}

func (s *SaturdayDisco) TemplateData() TemplateData {
	c := make(chan TemplateData)
	s.templateC <- c
	return <-c
}

func (s *SaturdayDisco) hasQuorum() bool {
	total := 0
	for _, participant := range s.Participants {
		total += participant.Count
	}
	return total >= QUORUM
}

func (s *SaturdayDisco) log(format string, args ...any) {
	s.logi(0, format, args...)
}

func (s *SaturdayDisco) logi(i uint, format string, args ...any) {
	out := say.F("{{gray}}[%s]{{/}} SaturdayDisco: ", s.alarmClock.Time().Format("1/2 3:04:05am"))
	out += say.Fi(i, format, args...) + "\n"
	s.w.Write([]byte(out))
}

func (s *SaturdayDisco) emailData() TemplateData {
	forecast, err := s.forecaster.ForecastFor(s.T)
	if err != nil {
		s.logi(0, "{{red}}failed to fetch forecast: %s{{/}}", err.Error())
		forecast = weather.Forecast{}
	}
	return TemplateData{
		GameDate:              s.T.Format("1/2"),
		GameTime:              s.T.Format("3:04pm"),
		SaturdayDiscoSnapshot: s.SaturdayDiscoSnapshot,
		DiscoEmailAddress:     s.config.SaturdayDiscoEmail.String(),
		HasQuorum:             s.hasQuorum(),
		GameOn:                s.State == StateGameOnSent || s.State == StateReminderSent,
		GameOff:               s.State == StateNoInviteSent || s.State == StateNoGameSent,
		Forecast:              forecast,
	}.WithNextEvent(s.NextEvent)
}

func (s *SaturdayDisco) emailSubject(name string, data TemplateData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_subject", data)
	return b.String()
}

func (s *SaturdayDisco) emailBody(name string, data TemplateData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_body", data)
	return b.String()
}

func (s *SaturdayDisco) emailForBoss(name string, data TemplateData) mail.Email {
	return mail.E().
		WithFrom(s.config.SaturdayDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject(s.emailSubject(name, data)).
		WithBody(s.emailBody(name, data))
}

func (s *SaturdayDisco) emailForList(name string, data TemplateData) mail.Email {
	return mail.E().
		WithFrom(s.config.SaturdayDiscoEmail).
		WithTo(s.config.SaturdayDiscoList).
		WithSubject(s.emailSubject(name, data)).
		WithBody(mail.Markdown(s.emailBody(name, data)))
}

func (s *SaturdayDisco) dance() {
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
			c <- s.SaturdayDiscoSnapshot.dup()
		}
	}
}

func (s *SaturdayDisco) backup() {
	s.log("{{yellow}}backing up...{{/}}")
	data, err := json.Marshal(s.SaturdayDiscoSnapshot)
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

var setCommandRegex = regexp.MustCompile(`^/set\s+(.+)+\s+(\d+)$`)
var delayCommandRegex = regexp.MustCompile(`^/delay\s+(\d+)$`)

func (s *SaturdayDisco) processEmail(email mail.Email) {
	s.logi(0, "{{yellow}}Processing Email:{{/}}")
	s.logi(1, "Email: %s", email.String())
	c := Command{Email: email}
	isFromSelf := email.From.Equals(s.config.SaturdayDiscoEmail)
	if isFromSelf {
		return
	}

	isAdminCommand := email.From.Equals(s.config.BossEmail) &&
		len(email.To) == 1 &&
		len(email.CC) == 0 &&
		email.To[0].Equals(s.config.SaturdayDiscoEmail)
	isAdminReply := isAdminCommand && strings.HasPrefix(email.Subject, "Re: [")
	isPotentialPlayerCommand := !email.From.Equals(s.config.BossEmail)

	var err error
	if isAdminReply {
		if strings.HasPrefix(email.Subject, "Re: [invite-approval-request]") {
			c.CommandType = CommandRequestedInviteApprovalReply
		} else if strings.HasPrefix(email.Subject, "Re: [badger-approval-request]") {
			c.CommandType = CommandRequestedBadgerApprovalReply
		} else if strings.HasPrefix(email.Subject, "Re: [game-on-approval-request]") {
			c.CommandType = CommandRequestedGameOnApprovalReply
		} else if strings.HasPrefix(email.Subject, "Re: [no-game-approval-request]") {
			c.CommandType = CommandRequestedNoGameApprovalReply
		} else {
			c.Error = fmt.Errorf("invalid reply subject: %s", email.Subject)
		}
		if c.Error == nil {
			commandLine := strings.Split(strings.TrimSpace(email.Text), "\n")[0]
			if strings.HasPrefix(commandLine, "/approve") || strings.HasPrefix(commandLine, "/yes") || strings.HasPrefix(commandLine, "/shipit") {
				c.Approved = true
			} else if strings.HasPrefix(commandLine, "/deny") || strings.HasPrefix(commandLine, "/no") {
				c.Approved = false
			} else if match := delayCommandRegex.FindAllStringSubmatch(commandLine, -1); match != nil {
				c.Delay, err = strconv.Atoi(match[0][1])
				if c.Delay <= 0 {
					c.Error = fmt.Errorf("invalid delay count for /delay command: %s - must be > 0", match[0][1])
				}
				if err != nil {
					c.Error = fmt.Errorf("invalid delay count for /delay command: %s", match[0][1])
				}
			} else if strings.HasPrefix(commandLine, "/RESET-RESET-RESET") {
				c.CommandType = CommandAdminReset
			} else if strings.HasPrefix(commandLine, "/abort") {
				c.CommandType = CommandAdminAbort
			} else {
				c.Error = fmt.Errorf("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, /no, /delay <int>, /abort, or /RESET-RESET-RESET")
			}
		}
		if c.Error == nil {
			idxFirstNewline := strings.Index(email.Text, "\n")
			if idxFirstNewline > -1 {
				c.AdditionalContent = strings.Trim(email.Text[idxFirstNewline:], "\n")
			}
		} else {
			c.CommandType = CommandInvalidReply
		}
	} else if isAdminCommand {
		commandLine := strings.Split(strings.TrimSpace(email.Text), "\n")[0]
		if match := setCommandRegex.FindAllStringSubmatch(commandLine, -1); match != nil {
			c.CommandType = CommandAdminSetCount
			c.EmailAddress = mail.EmailAddress(match[0][1])
			c.Count, err = strconv.Atoi(match[0][2])
			if err != nil {
				c.Error = fmt.Errorf("invalid count for /set command: %s", match[0][2])
			}
		} else if strings.HasPrefix(commandLine, "/status") {
			c.CommandType = CommandAdminStatus
		} else if strings.HasPrefix(commandLine, "/debug") {
			c.CommandType = CommandAdminDebug
		} else if strings.HasPrefix(commandLine, "/abort") {
			c.CommandType = CommandAdminAbort
		} else if strings.HasPrefix(email.Text, "/RESET-RESET-RESET") {
			c.CommandType = CommandAdminReset
		} else if strings.HasPrefix(commandLine, "/game-on") {
			c.CommandType = CommandAdminGameOn
			idxFirstNewline := strings.Index(email.Text, "\n")
			if idxFirstNewline > -1 {
				c.AdditionalContent = strings.Trim(email.Text[idxFirstNewline:], "\n")
			}
		} else if strings.HasPrefix(commandLine, "/no-game") {
			c.CommandType = CommandAdminNoGame
			idxFirstNewline := strings.Index(email.Text, "\n")
			if idxFirstNewline > -1 {
				c.AdditionalContent = strings.Trim(email.Text[idxFirstNewline:], "\n")
			}
		} else {
			c.Error = fmt.Errorf("invalid command: %s", commandLine)
		}
		if c.Error != nil {
			c.CommandType = CommandAdminInvalid
		}
	} else if isPotentialPlayerCommand {
		potentialCommand, err := s.interpreter.InterpretEmail(email, s.Participants.CountFor(email.From))
		if err != nil {
			c.CommandType = CommandPlayerError
			c.Error = err
		} else {
			c = potentialCommand
		}
	} else {
		//this is not a command - do nothing
		return
	}

	if c.Error != nil {
		s.logi(1, "{{red}}unable to extract command from email: %s - %s{{/}}", c.CommandType, c.Error.Error())
	}

	s.commandC <- c
}

func (s *SaturdayDisco) transitionTo(state SaturdayDiscoState) {
	switch state {
	case StatePending:
		s.NextEvent = s.T.Add(-4*day - 4*time.Hour) //Tuesday, 6am
	case StateInviteSent:
		s.NextEvent = s.T.Add(-2*day + 4*time.Hour) //Thursday, 2pm
	case StateBadgerSent, StateBadgerNotSent:
		s.NextEvent = s.T.Add(-day - 4*time.Hour) //Friday, 6am
	case StateRequestedInviteApproval, StateRequestedBadgerApproval, StateRequestedGameOnApproval, StateRequestedNoGameApproval:
		s.NextEvent = s.alarmClock.Time().Add(ApprovalTime) //you get 4 hours to reply, Boss
	case StateGameOnSent:
		s.NextEvent = s.T.Add(-4 * time.Hour) //Saturday, 6am
	case StateNoInviteSent, StateNoGameSent, StateReminderSent, StateAbort:
		s.NextEvent = s.T.Add(2 * time.Hour) //Saturday, 12pm is when we reset
	}
	s.State = state
	if !s.NextEvent.IsZero() {
		s.alarmClock.SetAlarm(s.NextEvent)
	}
}

func (s *SaturdayDisco) performNextEvent() {
	data := s.emailData()
	switch s.State {
	case StatePending:
		s.logi(1, "{{coral}}sending invite approval request to boss{{/}}")
		s.sendEmail(s.emailForBoss("request_invite_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
			StateRequestedInviteApproval, s.retryNextEventErrorHandler)

	case StateRequestedInviteApproval:
		s.logi(1, "{{green}}time's up, sending invitation e-mail{{/}}")
		s.sendEmail(s.emailForList("invitation", data),
			StateInviteSent, s.retryNextEventErrorHandler)
	case StateInviteSent:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}sending badger approval request to boss{{/}}")
			s.sendEmail(s.emailForBoss("request_badger_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedBadgerApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedBadgerApproval:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{green}}time's up, sending badger e-mail{{/}}")
			s.sendEmail(s.emailForList("badger", data),
				StateBadgerSent, s.retryNextEventErrorHandler)
		}
	case StateBadgerSent, StateBadgerNotSent:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}we still don't have quorum.  asking for permission to send no-game{{/}}")
			s.sendEmail(s.emailForBoss("request_no_game_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedGameOnApproval:
		if s.hasQuorum() {
			s.logi(1, "{{green}}we have quorum and time's up! sending game-on{{/}}")
			s.sendEmail(s.emailForList("game_on", data),
				StateGameOnSent, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}we lost quorum.  asking for permission to send no-game{{/}}")
			s.sendEmail(s.emailForBoss("request_no_game_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedNoGameApproval:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{green}}time's up, sending no-game e-mail{{/}}")
			s.sendEmail(s.emailForList("no_game", data),
				StateNoGameSent, s.retryNextEventErrorHandler)
		}
	case StateGameOnSent:
		s.sendEmail(s.emailForList("reminder", data), StateReminderSent, s.retryNextEventErrorHandler)
	case StateNoInviteSent, StateNoGameSent, StateReminderSent, StateAbort:
		s.reset()
	}
}

func (s *SaturdayDisco) handleCommand(command Command) {
	if s.ProcessedEmailIDs.Contains(command.Email.MessageID) {
		s.logi(1, "{{coral}}I've already processed this email (id: %s).  Ignoring.{{/}}", command.Email.MessageID)
		return
	}
	defer func() {
		s.ProcessedEmailIDs = append(s.ProcessedEmailIDs, command.Email.MessageID)
	}()
	switch command.CommandType {
	case CommandRequestedInviteApprovalReply, CommandRequestedBadgerApprovalReply, CommandRequestedGameOnApprovalReply, CommandRequestedNoGameApprovalReply:
		s.handleReplyCommand(command)
	case CommandInvalidReply:
		s.logi(1, "{{red}}boss sent me an invalid reply{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_admin_email", s.emailData().WithError(command.Error))))
	case CommandAdminStatus:
		s.logi(1, "{{green}}boss is asking for status{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("boss_status", s.emailData())))
	case CommandAdminReset:
		s.logi(1, "{{red}}BOSS IS RESETTING THE SYSTEM.  HOLD ON TO YOUR BUTTS.{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("reset", s.emailData())))
		s.reset()
	case CommandAdminDebug:
		s.logi(1, "{{green}}boss is asking for debug info{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			mail.Markdown(s.emailBody("boss_debug",
				s.emailData().
					WithMessage("Here's what a **multiline message** looks like.\n\n_Woohoo!_").
					WithError(fmt.Errorf("And this is what an error looks like!"))),
			)))
	case CommandAdminAbort:
		s.logi(1, "{{red}}boss has asked me to abort{{/}}")
		s.sendEmail(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("abort", s.emailData())),
			StateAbort, s.replyWithFailureErrorHandler)
	case CommandAdminGameOn:
		s.logi(1, "{{green}}boss has asked me to send game-on{{/}}")
		s.sendEmail(s.emailForList("game_on",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateGameOnSent, s.replyWithFailureErrorHandler)
	case CommandAdminNoGame:
		s.logi(1, "{{red}}boss has asked me to send no-game{{/}}")
		s.sendEmail(s.emailForList("no_game",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoGameSent, s.replyWithFailureErrorHandler)
	case CommandAdminSetCount:
		s.logi(1, "{{green}}boss has asked me to adjust a participant count{{/}}")
		s.logi(2, "{{gray}}Setting %s to %d{{/}}", command.EmailAddress, command.Count)
		s.Participants = s.Participants.UpdateCount(command.EmailAddress, command.Count, command.Email)
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("acknowledge_admin_set_count",
				s.emailData().WithMessage("%s to %d", command.EmailAddress, command.Count))))
	case CommandAdminInvalid:
		s.logi(1, "{{red}}boss sent me an invalid command{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_admin_email",
				s.emailData().WithError(command.Error))))
	case CommandPlayerSetCount:
		s.logi(1, "{{green}}player sent a message signing up.{{/}}")
		s.logi(2, "{{gray}}Setting %s to %d{{/}}", command.EmailAddress, command.Count)
		s.Participants = s.Participants.UpdateCount(command.EmailAddress, command.Count, command.Email)
		s.sendEmailWithNoTransition(command.Email.Forward(s.config.SaturdayDiscoEmail, s.config.BossEmail,
			mail.Markdown(s.emailBody("acknowledge_player_set_count", s.emailData().WithMessage("%d", command.Count).WithAttachment(command.EmailAddress).WithEmailDebugKey(command.Email.DebugKey)))))
	case CommandPlayerIgnore:
		s.logi(1, "{{yellow}}ignoring this e-mail{{/}}")
	case CommandPlayerError:
		s.logi(1, "{{red}}encountered an error while processing a player command: %s{{/}}", command.Error.Error())
		s.sendEmailWithNoTransition(command.Email.Forward(s.config.SaturdayDiscoEmail, s.config.BossEmail,
			s.emailBody("error_player_command", s.emailData().WithError(command.Error))))
	}
}

func (s *SaturdayDisco) handleReplyCommand(command Command) {
	data := s.emailData().WithMessage(command.AdditionalContent).WithError(command.Error)
	var expectedState SaturdayDiscoState
	var requestedApproval string
	switch command.CommandType {
	case CommandRequestedInviteApprovalReply:
		expectedState = StateRequestedInviteApproval
		requestedApproval = "invite"
	case CommandRequestedBadgerApprovalReply:
		expectedState = StateRequestedBadgerApproval
		requestedApproval = "badger"
	case CommandRequestedGameOnApprovalReply:
		expectedState = StateRequestedGameOnApproval
		requestedApproval = "game on"
	case CommandRequestedNoGameApprovalReply:
		expectedState = StateRequestedNoGameApproval
		requestedApproval = "no game"
	}
	if s.State != expectedState {
		s.logi(1, "{{red}}boss sent me a reply command: %s, but i'm in the wrong state: %s{{/}}", command.CommandType, s.State)
		s.sendEmailWithNoTransition(command.Email.Reply(
			s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_reply_state_email", data),
		))
		return
	}
	if command.Delay > 0 {
		s.logi(1, "{{green}}boss says to delay the next event by %d hours{{/}}", command.Delay)
		s.NextEvent = s.NextEvent.Add(time.Duration(command.Delay) * time.Hour)
		s.alarmClock.SetAlarm(s.NextEvent)
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("acknowledge_delay", s.emailData().WithMessage("the %s email by %d hours", requestedApproval, command.Delay))))
		return
	}

	switch command.CommandType {
	case CommandRequestedInviteApprovalReply:
		if command.Approved {
			s.logi(1, "{{green}}boss says it's ok to send the invite, sending invitation e-mail{{/}}")
			s.sendEmail(s.emailForList("invitation", data),
				StateInviteSent, s.replyWithFailureErrorHandler)
		} else {
			s.logi(1, "{{orange}}boss says it's not ok to send the invite, sending no-invitation e-mail{{/}}")
			s.sendEmail(s.emailForList("no_invitation", data),
				StateNoInviteSent, s.replyWithFailureErrorHandler)
		}
	case CommandRequestedBadgerApprovalReply:
		if command.Approved {
			s.logi(1, "{{green}}boss says it's ok to send the badger, sending badger e-mail{{/}}")
			s.sendEmail(s.emailForList("badger", data),
				StateBadgerSent, s.replyWithFailureErrorHandler)
		} else {
			s.logi(1, "{{red}}boss says not to badger folks, so i won't{{/}}")
			s.transitionTo(StateBadgerNotSent)
		}
	case CommandRequestedGameOnApprovalReply:
		if command.Approved {
			if s.hasQuorum() {
				s.logi(1, "{{green}}boss says it's ok to send game on, sending game-on e-mail{{/}}")
				s.sendEmail(s.emailForList("game_on", data),
					StateGameOnSent, s.replyWithFailureErrorHandler)
			} else {
				s.logi(1, "{{red}}boss says it's ok to send game on, but we don't have quorum, sending error email then no-game approval request{{/}}")
				s.sendEmailWithNoTransition(command.Email.Reply(
					s.config.SaturdayDiscoEmail,
					s.emailBody("invalid_admin_email", data.WithError(fmt.Errorf("Quorum was lost before this approval came in.  Starting the No-Game flow soon."))),
				))
				s.sendEmail(s.emailForBoss("request_no_game_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
					StateRequestedNoGameApproval, s.replyWithFailureErrorHandler)
			}
		} else {
			s.logi(1, "{{green}}boss says it's not ok to send game on, sending no-game e-mail{{/}}")
			s.sendEmail(s.emailForList("no_game", data),
				StateNoGameSent, s.replyWithFailureErrorHandler)
		}
	case CommandRequestedNoGameApprovalReply:
		if s.hasQuorum() {
			s.logi(1, "{{red}}boss says it's ok to send no game, but we have quorum now, sending error email then no-game approval request{{/}}")
			s.sendEmailWithNoTransition(command.Email.Reply(
				s.config.SaturdayDiscoEmail,
				s.emailBody("invalid_admin_email", data.WithError(fmt.Errorf("Quorum was gained before this came in.  Starting the Game-On flow soon."))),
			))
			s.sendEmail(s.emailForBoss("request_game_on_approval", data.WithNextEvent(s.alarmClock.Time().Add(ApprovalTime))),
				StateRequestedGameOnApproval, s.replyWithFailureErrorHandler)
		} else {
			if command.Approved {
				s.logi(1, "{{green}}boss says it's ok to send no game, sending no-game e-mail{{/}}")
				s.sendEmail(s.emailForList("no_game", data),
					StateNoGameSent, s.replyWithFailureErrorHandler)
			} else {
				s.logi(1, "{{green}}boss says not to send the no-game email so i'm aborting{{/}}")
				s.sendEmail(command.Email.Reply(
					s.config.SaturdayDiscoEmail,
					s.emailBody("abort", data)),
					StateAbort, s.replyWithFailureErrorHandler)

			}
		}
	}
}

func (s *SaturdayDisco) retryNextEventErrorHandler(email mail.Email, err error) {
	s.outbox.SendEmail(mail.Email{
		From:    s.config.SaturdayDiscoEmail,
		To:      []mail.EmailAddress{s.config.BossEmail},
		Subject: "Help!",
		Text:    fmt.Sprintf("Saturday Disco failed to send an e-mail during an event transition.\n\n%s\n\nTrying to send:\n\n%s\n\nPlease help!", err.Error(), email.String()),
	})
	s.alarmClock.SetAlarm(s.alarmClock.Time().Add(RETRY_DELAY))
}

func (s *SaturdayDisco) replyWithFailureErrorHandler(email mail.Email, err error) {
	s.logi(1, "{{red}}failed while handling a command: %s{{/}}", err.Error())
	s.outbox.SendEmail(mail.Email{
		From:    s.config.SaturdayDiscoEmail,
		To:      []mail.EmailAddress{s.config.BossEmail},
		Subject: "Help!",
		Text:    fmt.Sprintf("Saturday Disco failed while trying to handle a command.\n\n%s\n\nTrying to send:\n\n%s\n\nPlease help!", err.Error(), email.String()),
	})
}

func (s *SaturdayDisco) sendEmail(email mail.Email, successState SaturdayDiscoState, onFailure func(mail.Email, error)) {
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

func (s *SaturdayDisco) sendEmailWithNoTransition(email mail.Email) {
	err := s.outbox.SendEmail(email)
	if err != nil {
		s.logi(1, "{{red}}failed to send e-mail: %s{{/}}", err.Error())
		s.logi(2, "email: %s", email)
	}
}

func (s *SaturdayDisco) reset() {
	s.alarmClock.Stop()
	s.State = StateInvalid
	s.Participants = Participants{}
	s.T = clock.NextSaturdayAt10Or1030(s.alarmClock.Time())
	s.NextEvent = time.Time{}
	s.ProcessedEmailIDs = ProcessedEmailIDs{}
	s.transitionTo(StatePending)
}
