package saturdaydisco

import (
	"context"
	"embed"
	"fmt"
	"io"
	"regexp"
	"strconv"
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

	EmailAddress mail.EmailAddress
	Count        int

	Error error

	Attempts int
}

type Participant struct {
	Address        mail.EmailAddress
	Count          int
	RelevantEmails []mail.Email
}

func (p Participant) IndentedRelevantEmails() string {
	out := &strings.Builder{}
	for idx, email := range p.RelevantEmails {
		say.Fpiw(out, 2, 100, "%s\n", email.String())
		if idx < len(p.RelevantEmails)-1 {
			say.Fpi(out, 2, "---\n")
		}
	}
	return out.String()
}

type Participants []Participant

func (p Participants) UpdateCount(address mail.EmailAddress, count int, relevantEmail mail.Email) Participants {
	for i := range p {
		if p[i].Address.Equals(address) {
			if !p[i].Address.HasExplicitName() {
				p[i].Address = address
			}
			p[i].Count = count
			p[i].RelevantEmails = append(p[i].RelevantEmails, relevantEmail)
			return p
		}
	}
	return append(p, Participant{
		Address:        address,
		Count:          count,
		RelevantEmails: []mail.Email{relevantEmail},
	})
}

func (p Participants) Count() int {
	total := 0
	for _, participant := range p {
		total += participant.Count
	}
	return total
}

func (p Participants) Public() string {
	if p.Count() == 0 {
		return "No one's signed up yet"
	}

	out := &strings.Builder{}
	for i, participant := range p {
		if p.Count() == 0 {
			continue
		}
		out.WriteString(participant.Address.Name())
		if participant.Count > 1 {
			fmt.Fprintf(out, " **(%d)**", participant.Count)
		}
		if i < len(p)-2 {
			out.WriteString(", ")
		} else if i == len(p)-2 {
			out.WriteString(" and ")
		}
	}
	return out.String()
}

func (p Participants) dup() Participants {
	participants := make(Participants, len(p))
	copy(participants, p)
	return participants
}

type SaturdayDiscoSnapshot struct {
	State        SaturdayDiscoState `json:"state"`
	Participants Participants       `json:"participants"`
	NextEvent    time.Time          `json:"next_event"`
	T            time.Time          `json:"reference_time"`
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

	alarmClock AlarmClockInt
	outbox     mail.OutboxInt
	commandC   chan Command
	snapshotC  chan chan<- SaturdayDiscoSnapshot
	config     config.Config
	lock       *sync.Mutex
	ctx        context.Context
	cancel     func()
}

type TemplateData struct {
	GameDate string
	GameTime string
	SaturdayDiscoSnapshot
	HasQuorum bool

	Message string
	Error   error
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

func NewSaturdayDisco(config config.Config, w io.Writer, alarmClock AlarmClockInt, outbox mail.OutboxInt) *SaturdayDisco {
	saturdayDisco := &SaturdayDisco{
		alarmClock: alarmClock,
		outbox:     outbox,
		commandC:   make(chan Command),
		snapshotC:  make(chan chan<- SaturdayDiscoSnapshot),
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

func (s *SaturdayDisco) GetSnapshot() SaturdayDiscoSnapshot {
	c := make(chan SaturdayDiscoSnapshot)
	s.snapshotC <- c
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
	return TemplateData{
		GameDate:              s.T.Format("1/2/06"),
		GameTime:              s.T.Format("3:04pm"),
		SaturdayDiscoSnapshot: s.SaturdayDiscoSnapshot,
		HasQuorum:             s.hasQuorum(),
	}
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
		case command := <-s.commandC:
			s.log("{{yellow}}received a command{{/}}")
			s.handleCommand(command)
		case c := <-s.snapshotC:
			c <- s.SaturdayDiscoSnapshot.dup()
		}
	}
}

var setCommandRegex = regexp.MustCompile(`^/set\s+(.+)+\s+(\d+)$`)

func (s *SaturdayDisco) processEmail(email mail.Email) {
	s.logi(0, "{{yellow}}Processing Email:{{/}}")
	s.logi(1, "Email: %s", email.String())
	c := Command{Email: email}
	isAdminReply := email.From.Equals(s.config.BossEmail) &&
		len(email.To) == 1 &&
		len(email.CC) == 0 &&
		email.To[0].Equals(s.config.SaturdayDiscoEmail) &&
		strings.HasPrefix(email.Subject, "Re: [")
	isAdminCommand := email.From.Equals(s.config.BossEmail) &&
		!email.IncludesRecipient(s.config.SaturdayDiscoList) &&
		email.Text != ""
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
			if strings.HasPrefix(email.Text, "/approve") || strings.HasPrefix(email.Text, "/yes") || strings.HasPrefix(email.Text, "/shipit") {
				c.Approved = true
			} else if strings.HasPrefix(email.Text, "/deny") || strings.HasPrefix(email.Text, "/no") {
				c.Approved = false
			} else {
				c.Error = fmt.Errorf("invalid command in reply, must be one of /approve, /yes, /shipit, /deny, or /no")
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
		} else if strings.HasPrefix(commandLine, "/abort") {
			c.CommandType = CommandAdminAbort
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
			c.Error = fmt.Errorf("could not extract valid command from: %s", commandLine)
		}
		if c.Error != nil {
			c.CommandType = CommandAdminInvalid
		}
	} else if isPotentialPlayerCommand {
		//TODO
	} else {
		//this is not a command
		return
	}

	if c.Error != nil {
		s.logi(1, "{{red}}unable to extract command from email: %s{{/}}", c.Error.Error())
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
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}sending badger approval request to boss{{/}}")
			s.sendEmail(s.emailForBoss("request_badger_approval", data),
				StateRequestedBadgerApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedBadgerApproval:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{green}}time's up, sending badger e-mail{{/}}")
			s.sendEmail(s.emailForList("badger", data),
				StateBadgerSent, s.retryNextEventErrorHandler)
		}
	case StateBadgerSent, StateBadgerNotSent:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data),
				StateRequestedGameOnApproval, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}we still don't have quorum.  asking for permission to send no-game{{/}}")
			s.sendEmail(s.emailForBoss("request_no_game_approval", data),
				StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedGameOnApproval:
		if s.hasQuorum() {
			s.logi(1, "{{green}}we have quorum and time's up! sending game-on{{/}}")
			s.sendEmail(s.emailForList("game_on", data),
				StateGameOnSent, s.retryNextEventErrorHandler)
		} else {
			s.logi(1, "{{coral}}we lost quorum.  asking for permission to send no-game{{/}}")
			s.sendEmail(s.emailForBoss("request_no_game_approval", data),
				StateRequestedNoGameApproval, s.retryNextEventErrorHandler)
		}
	case StateRequestedNoGameApproval:
		if s.hasQuorum() {
			s.logi(1, "{{coral}}we have quorum!  asking for permission to send game-on{{/}}")
			s.sendEmail(s.emailForBoss("request_game_on_approval", data),
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
	switch command.CommandType {
	case CommandRequestedInviteApprovalReply, CommandRequestedBadgerApprovalReply, CommandRequestedGameOnApprovalReply, CommandRequestedNoGameApprovalReply:
		s.handleReplyCommand(command)
	case CommandInvalidReply:
		s.logi(1, "{{red}}boss sent me an invalid reply{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_admin_email", s.emailData().WithError(command.Error))))
	case CommandAdminStatus:
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("boss_status", s.emailData())))
	case CommandAdminAbort:
		s.sendEmail(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("abort", s.emailData())),
			StateAbort, s.replyWithFailureErrorHandler)
	case CommandAdminGameOn:
		s.sendEmail(s.emailForList("game_on",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateGameOnSent, s.replyWithFailureErrorHandler)
	case CommandAdminNoGame:
		s.sendEmail(s.emailForList("no_game",
			s.emailData().WithMessage(command.AdditionalContent)),
			StateNoGameSent, s.replyWithFailureErrorHandler)
	case CommandAdminSetCount:
		s.logi(1, "{{green}}boss has asked me to adjust a participant count{{/}}")
		s.Participants = s.Participants.UpdateCount(command.EmailAddress, command.Count, command.Email)
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("acknowledge_admin_set_count",
				s.emailData().WithMessage("%s to %d", command.EmailAddress, command.Count))))
	case CommandAdminInvalid:
		s.logi(1, "{{red}}boss sent me an invalid command{{/}}")
		s.sendEmailWithNoTransition(command.Email.Reply(s.config.SaturdayDiscoEmail,
			s.emailBody("invalid_admin_email",
				s.emailData().WithError(command.Error))))
	case CommandPlayerStatus:
		//		s.sendEmailWithNoTransition("public-status-email") //reply-all cc boss too, has logic around things like "hasn't been called yet"
	case CommandPlayerUnsubscribe:
		//		s.sendEmailWithNoTransition("unsubscribe-requested") //to boss only
	case CommandPlayerSetCount:
		s.Participants = s.Participants.UpdateCount(command.EmailAddress, command.Count, command.Email)
		//		s.sendEmailWithNoTransition("acknowledge-player-set-count") //cc boss
	case CommandPlayerUnsure:
		if !command.Email.IncludesRecipient(s.config.SaturdayDiscoList) {
			//			s.sendEmailWithNoTransition("invalid-player-email") //to boss only
		}
	}
}

func (s *SaturdayDisco) handleReplyCommand(command Command) {
	data := s.emailData().WithMessage(command.AdditionalContent).WithError(command.Error)
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
				s.sendEmail(s.emailForBoss("request_no_game_approval", data),
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
			s.sendEmail(s.emailForBoss("request_game_on_approval", data),
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
	s.Participants = Participants{}
	s.T = NextSaturdayAt10(s.alarmClock.Time())
	s.NextEvent = time.Time{}
	s.transitionTo(StatePending)
}
