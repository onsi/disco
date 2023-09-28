package saturdaydisco

import (
	"context"
	"embed"
	"fmt"
	"io"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
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

	fmt.Println(templates.DefinedTemplates())

}

const QUORUM = 8

const day = 24 * time.Hour
const RETRY_DELAY = 5 * time.Minute

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
	CommandInvalid CommandType = `invalid`

	CommandRequestedInviteApprovalReply CommandType = `requested_invite_approval_reply`
	CommandRequestedBadgerApprovalReply CommandType = `requested_badger_approval_reply`
	CommandRequestedGameOnApprovalReply CommandType = "requested_game_on_approval_reply"
	CommandRequestedNoGameApprovalReply CommandType = "requested_no_game_approval_reply"
	CommandInvalidReply                 CommandType = "invalid_reply"

	CommandAdminStatus   CommandType = "admin_status"
	CommandAdminAbort    CommandType = "admin_abort"
	CommandAdminGameOn   CommandType = "admin_game_on"
	CommandAdminNoGame   CommandType = "admin_no_game"
	CommandAdminSetCount CommandType = "admin_set_count"
	CommandAdminInvalid  CommandType = "admin_invalid"

	CommandPlayerStatus      CommandType = "player_status"
	CommandPlayerUnsubscribe CommandType = "player_unsubscribe"
	CommandPlayerSetCount    CommandType = "player_set_count"
	CommandPlayerUnsure      CommandType = "player_unsure"
)

type Command struct {
	CommandType CommandType
	Email       mail.Email

	Approved          bool
	AdditionalContent string

	User  mail.EmailAddress
	Count int

	Error error

	Attempts int
}

type SaturdayDisco struct {
	State     SaturdayDiscoState `json:"state"`
	Count     map[string]int     `json:"count"`
	NextEvent time.Time          `json:"next_event"`
	T         time.Time          `json:"reference_time"`
	w         io.Writer

	alarmClock AlarmClockInt
	outbox     mail.OutboxInt
	commandC   chan Command
	config     config.Config
	lock       *sync.Mutex
	ctx        context.Context
	cancel     func()
}

type EmailData struct {
	GameDate string
	GameTime string
	State    SaturdayDiscoState

	AdditionalContent string
}

func (e EmailData) WithAdditionalContent(content string) EmailData {
	e.AdditionalContent = content
	return e
}
func NewSaturdayDisco(config config.Config, w io.Writer, alarmClock AlarmClockInt, outbox mail.OutboxInt) *SaturdayDisco {
	saturdayDisco := &SaturdayDisco{
		alarmClock: alarmClock,
		outbox:     outbox,
		commandC:   make(chan Command),
		w:          w,

		config: config,
		lock:   &sync.Mutex{},
	}
	saturdayDisco.ctx, saturdayDisco.cancel = context.WithCancel(context.Background())

	//load goes here
	//otherwise:
	saturdayDisco.reset()
	go saturdayDisco.dance()

	return saturdayDisco
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

func (s *SaturdayDisco) HasQuorum() bool {
	total := 0
	for _, count := range s.Count {
		total += count
	}
	return total >= QUORUM
}

func (s *SaturdayDisco) log(format string, args ...any) {
	s.logi(0, format, args...)
}

func (s *SaturdayDisco) logi(i uint, format string, args ...any) {
	say.Fplni(s.w, i, "{{gray}}[%s]{{/}} SaturdayDisco: "+format, append([]any{s.alarmClock.Time().Format("1/2 3:04:05am")}, args...))
}

func (s *SaturdayDisco) emailData() EmailData {
	return EmailData{
		GameDate: s.T.Format("1/2/06"),
		GameTime: s.T.Format("3:04pm"),
		State:    s.State,
	}
}

func (s *SaturdayDisco) emailSubject(name string, data EmailData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_subject", data)
	return b.String()
}

func (s *SaturdayDisco) emailBody(name string, data EmailData) string {
	b := &strings.Builder{}
	templates.ExecuteTemplate(b, name+"_body", data)
	return b.String()
}

func (s *SaturdayDisco) emailForBoss(name string, data EmailData) mail.Email {
	return mail.E().
		WithFrom(s.config.SaturdayDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject(s.emailSubject(name, data)).
		WithBody(s.emailBody(name, data))
}

func (s *SaturdayDisco) emailForList(name string, data EmailData) mail.Email {
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
		case command := <-s.commandC:
			s.log("{{yellow}}received a command{{/}}")
			s.handleCommand(command)
		}
	}
}

func (s *SaturdayDisco) processEmail(email mail.Email) {
	c := Command{Email: email}
	isAdminReply := email.From.Equals(s.config.BossEmail) &&
		len(email.To) == 1 &&
		len(email.CC) == 0 &&
		email.To[0].Equals(s.config.SaturdayDiscoEmail) &&
		strings.HasPrefix(email.Subject, "Re: [")
	if isAdminReply {
		if strings.HasPrefix(email.Subject, "Re: [invite-approval-request]") {
			c.CommandType = CommandRequestedInviteApprovalReply
		} else {
			c.CommandType = CommandInvalidReply
			c.Error = fmt.Errorf("invalid reply subject: %s", email.Subject)
		}
		if c.CommandType != CommandInvalidReply {
			if strings.HasPrefix(email.Text, "/approve") || strings.HasPrefix(email.Text, "/yes") || strings.HasPrefix(email.Text, "/shipit") {
				c.Approved = true
			} else if strings.HasPrefix(email.Text, "/deny") || strings.HasPrefix(email.Text, "/no") {
				c.Approved = false
			} else {
				c.CommandType = CommandInvalidReply
				c.Error = fmt.Errorf("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, or /no")
			}
		}
		if c.CommandType != CommandInvalidReply {
			idxFirstNewline := strings.Index(email.Text, "\n")
			if idxFirstNewline > -1 {
				c.AdditionalContent = strings.Trim(email.Text[idxFirstNewline:], "\n")
			}
		}
	} else {
		c.CommandType = CommandInvalid
		c.Error = fmt.Errorf("unable to parse command")
	}

	s.commandC <- c
}

func (s *SaturdayDisco) transitionTo(state SaturdayDiscoState) {
	switch state {
	//TODO: make sure i've got 'em all!
	case StatePending:
		s.NextEvent = s.T.Add(-4*day - 4*time.Hour) //Tuesday, 6am
	case StateInviteSent:
		s.NextEvent = s.T.Add(-2*day + 4*time.Hour) //Thursday, 2pm
	case StateBadgerSent, StateBadgerNotSent:
		s.NextEvent = s.T.Add(-day - 4*time.Hour) //Friday, 6am
	case StateRequestedInviteApproval, StateRequestedBadgerApproval, StateRequestedGameOnApproval, StateRequestedNoGameApproval:
		s.NextEvent = s.alarmClock.Time().Add(4 * time.Hour) //you get 4 hours to reply, Boss
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
		s.sendEmail(s.emailForBoss("request_invite_approval", data),
			StateRequestedInviteApproval, s.retryNextEventErrorHandler)
	case StateRequestedInviteApproval:
		s.logi(1, "{{green}}time's up, sending invitation e-mail{{/}}")
		s.sendEmail(s.emailForList("invitation", data),
			StateInviteSent, s.retryNextEventErrorHandler)
	case StateInviteSent:
		if s.HasQuorum() {
			// s.sendEmail("request-game-on-approval-email", StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			// s.sendEmail("request-badger-approval-email", StateRequestedBadgerApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedBadgerApproval:
		if s.HasQuorum() {
			// s.sendEmail("request-game-on-approval-email", StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			// s.sendEmail("badger-email", StateBadgerSent, s.retryNextEventErrorHandler)
		}
	case StateBadgerSent, StateBadgerNotSent:
		if s.HasQuorum() {
			// s.sendEmail("request-game-on-approval-email", StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			// s.sendEmail("request-no-game-approval-email", StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedGameOnApproval:
		if s.HasQuorum() {
			// s.sendEmail("game-on-email", StateGameOnSent, s.retryNextEventErrorHandler)
		} else {
			// s.sendEmail("request-no-game-approval-email", StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedNoGameApproval:
		if s.HasQuorum() {
			// s.sendEmail("request-game-on-approval-email", StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			// s.sendEmail("no-game-email", StateNoGameSent, s.retryNextEventErrorHandler)
		}
	case StateGameOnSent:
		// s.sendEmail("reminder-email", StateReminderSent, s.retryNextEventErrorHandler)
	case StateNoInviteSent, StateNoGameSent, StateReminderSent, StateAbort:
		s.reset()
	}
}

func (s *SaturdayDisco) handleCommand(command Command) {
	switch command.CommandType {
	case CommandRequestedInviteApprovalReply, CommandRequestedBadgerApprovalReply, CommandRequestedGameOnApprovalReply, CommandRequestedNoGameApprovalReply:
		s.handleReplyCommand(command)
	case CommandInvalidReply:
		//		s.sendEmailWithNoTransition("invalid-reply-email")
	case CommandAdminStatus:
		//		s.sendEmailWithNoTransition("full-state-dump-email")
	case CommandAdminAbort:
		s.transitionTo(StateAbort)
		//		s.sendEmailWithNoTransition("acknowledge-abort")
	case CommandAdminGameOn:
		// s.sendEmail("game-on-email", StateGameOnSent, s.replyWithFailureErrorHandler)
	case CommandAdminNoGame:
		if command.AdditionalContent == "" {
			//			s.sendEmailWithNoTransition("invalid-no-game-email")
		} else {
			// s.sendEmail("no-game-email", StateNoGameSent, s.replyWithFailureErrorHandler)
		}
	case CommandAdminSetCount:
		s.Count[command.User.Address()] = command.Count
		//		s.sendEmailWithNoTransition("acknowledge-admin-set-count") //includes status
	case CommandAdminInvalid:
		//		s.sendEmailWithNoTransition("invalid-admin-email")
	case CommandPlayerStatus:
		//		s.sendEmailWithNoTransition("public-status-email") //reply-all cc boss too, has logic around things like "hasn't been called yet"
	case CommandPlayerUnsubscribe:
		//		s.sendEmailWithNoTransition("unsubscribe-requested") //to boss only
	case CommandPlayerSetCount:
		s.Count[command.User.Address()] = command.Count
		//		s.sendEmailWithNoTransition("acknowledge-player-set-count") //cc boss
	case CommandPlayerUnsure:
		if !command.Email.IncludesRecipient(s.config.SaturdayDiscoList) {
			//			s.sendEmailWithNoTransition("invalid-player-email") //to boss only
		}
	}
}

func (s *SaturdayDisco) handleReplyCommand(command Command) {
	data := s.emailData().WithAdditionalContent(command.AdditionalContent)
	var expectedState SaturdayDiscoState
	switch command.CommandType {
	case CommandRequestedInviteApprovalReply:
		expectedState = StateRequestedInviteApproval
	case CommandRequestedBadgerApprovalReply:
		expectedState = StateRequestedBadgerApproval
	case CommandRequestedGameOnApprovalReply:
		expectedState = StateRequestedGameOnApproval
	case CommandRequestedNoGameApprovalReply:
		expectedState = StateRequestedNoGameApproval
	}
	if s.State != expectedState {
		s.logi(1, "{{red}}boss sent me a reply command: %s, but i'm in the wrong state: %s{{/}}", command.CommandType, s.State)
		s.sendEmailWithNoTransition(command.Email.Reply(
			s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_reply_state_email", data),
		))
		return
	}
	if command.Attempts > 3 {
		//this is a reply to command.Email
		// s.sendEmailWithNoTransition("too-many-attempts-email")
		return
	}
	if command.Approved {
		switch command.CommandType {
		case CommandRequestedInviteApprovalReply:
			s.logi(1, "{{green}}boss says it's ok to send the invite, sending invitation e-mail{{/}}")
			s.sendEmail(s.emailForList("invitation", data),
				StateInviteSent, s.replyWithFailureErrorHandler)
		case CommandRequestedBadgerApprovalReply:
			// s.sendEmail("badger-email", StateBadgerSent, s.replyWithFailureErrorHandler)
		case CommandRequestedGameOnApprovalReply:
			// s.sendEmail("game-on-email", StateGameOnSent, s.replyWithFailureErrorHandler)
		case CommandRequestedNoGameApprovalReply:
			// s.sendEmail("no-game-email", StateNoGameSent, s.replyWithFailureErrorHandler)
		}
	} else {
		switch command.CommandType {
		case CommandRequestedInviteApprovalReply:
			s.logi(1, "{{orange}}boss says it's not ok to send the invite, sending no-invitation e-mail{{/}}")
			s.sendEmail(s.emailForList("no_invitation", data),
				StateNoInviteSent, s.replyWithFailureErrorHandler)
		case CommandRequestedBadgerApprovalReply:
			s.transitionTo(StateBadgerNotSent)
		case CommandRequestedGameOnApprovalReply:
			// s.sendEmail("no-game-email", StateNoGameSent, s.replyWithFailureErrorHandler)
		case CommandRequestedNoGameApprovalReply:
			s.transitionTo(StateAbort)
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
	s.alarmClock.SetAlarm(time.Now().Add(RETRY_DELAY))
}

func (s *SaturdayDisco) replyWithFailureErrorHandler(email mail.Email, err error) {
	//TODO
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
	s.Count = map[string]int{}
	s.T = NextSaturdayAt10(s.alarmClock.Time())
	s.NextEvent = time.Time{}
	s.transitionTo(StatePending)
}
