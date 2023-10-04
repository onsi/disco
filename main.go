package main

import (
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/saturdaydisco"
	"github.com/onsi/disco/weather"
	"github.com/onsi/say"
)

type Server struct {
	e             *echo.Echo
	config        config.Config
	outbox        mail.OutboxInt
	saturdayDisco *saturdaydisco.SaturdayDisco
	db            s3db.S3DBInt

	TempEmails []string
}

func main() {
	conf := config.LoadConfig()
	server := &Server{
		e:      echo.New(),
		config: conf,
	}
	log.Fatal(server.Start())
}

func (s *Server) Start() error {
	t := NewTemplateRenderer(s.config.IsDev())
	s.e.Renderer = t
	s.e.Logger.SetLevel(log.INFO)
	if s.config.IsDev() {
		s.e.Debug = true
	}

	var err error
	var saturdayDisco *saturdaydisco.SaturdayDisco

	if s.config.IsDev() {
		s.db = s3db.NewFakeS3DB()
		outbox := mail.NewFakeOutbox()
		outbox.EnableLogging(s.e.Logger.Output())
		s.outbox = outbox
		saturdayDisco, err = saturdaydisco.NewSaturdayDisco(
			s.config,
			s.e.Logger.Output(),
			saturdaydisco.NewAlarmClock(),
			s.outbox,
			saturdaydisco.NewInterpreter(),
			weather.NewForecaster(s.db),
			s.db,
		)
		// some fake data just so we can better inspect the web page
		saturdayDisco.SaturdayDiscoSnapshot = saturdaydisco.SaturdayDiscoSnapshot{
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
			T:         saturdayDisco.T,
		}
	} else {
		s.db, err = s3db.NewS3DB()
		if err != nil {
			return err
		}
		s.outbox = mail.NewOutbox(s.config.ForwardEmailKey)

		saturdayDisco, err = saturdaydisco.NewSaturdayDisco(
			s.config,
			s.e.Logger.Output(),
			saturdaydisco.NewAlarmClock(),
			s.outbox,
			saturdaydisco.NewInterpreter(),
			weather.NewForecaster(s.db),
			s.db,
		)
	}
	if err != nil {
		return err
	}
	s.saturdayDisco = saturdayDisco
	s.RegisterRoutes()
	return s.e.Start(":" + s.config.Port)
}

func (s *Server) RegisterRoutes() {
	s.e.Use(middleware.Logger())
	s.e.Static("/img", "img")
	s.e.GET("/", s.Index)
	s.e.POST("/incoming/"+s.config.IncomingEmailGUID, s.IncomingEmail)
	s.e.POST("/subscribe", s.Subscribe)
}

func (s *Server) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index", TemplateData{
		Saturday: s.saturdayDisco.TemplateData(),
	})
}

func (s *Server) IncomingEmail(c echo.Context) error {
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	email, err := mail.ParseIncomingEmail(s.db, data, s.e.Logger.Output())
	if err != nil {
		s.e.Logger.Errorf("failed to parse incoming email: %s", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}

	s.saturdayDisco.HandleIncomingEmail(email)
	return c.NoContent(http.StatusOK)
}

var subscribeTemplate = template.Must(template.New("subscribe").Parse(`Hey boss,

We just got a subscription request:

Email: {{.Email}}
Wants Saturday: {{.WantsSaturday}}
Wants Lunchtime: {{.WantsLunchtime}}
{{if .Message}}Message: {{.Message}}{{end}}

Thanks,

Disco 🪩`))

type SubscriptionRequest struct {
	Email          string `json:"email"`
	WantsSaturday  bool   `json:"wantsSaturday"`
	WantsLunchtime bool   `json:"wantsLunchtime"`
	Message        string `json:"message"`
}

func truncate(input string, maxLength int) string {
	if len(input) > maxLength {
		input = input[:maxLength] + "..."
	}
	return input
}

func (s *Server) Subscribe(c echo.Context) error {
	say.Fplni(s.e.Logger.Output(), 0, "{{green}}Got a subscription request{{/}}")
	var request SubscriptionRequest
	if err := c.Bind(&request); err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to bind request %s{{/}}", err.Error())
		return c.String(http.StatusBadRequest, err.Error())
	}

	request.Email = truncate(strings.TrimSpace(request.Email), 100)
	request.Message = truncate(strings.TrimSpace(request.Message), 1000)

	body := &strings.Builder{}
	err := subscribeTemplate.Execute(body, request)
	if err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to render email body %s{{/}}", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	err = s.outbox.SendEmail(mail.E().
		WithFrom(s.config.SaturdayDiscoEmail).
		WithTo(s.config.BossEmail).
		WithSubject("New Subscription Request").WithBody(body.String()))
	if err != nil {
		say.Fplni(s.e.Logger.Output(), 1, "{{red}}Failed to send email %s{{/}}", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	say.Fplni(s.e.Logger.Output(), 1, "{{green}}Sent email{{/}}")
	return c.NoContent(http.StatusOK)
}

type Template struct {
	reload    bool
	templates *template.Template
	lock      *sync.Mutex
}

type TemplateData struct {
	Saturday saturdaydisco.TemplateData
}

func NewTemplateRenderer(reload bool) *Template {
	return &Template{
		reload:    reload,
		templates: template.Must(template.ParseGlob("html/*.html")),
		lock:      &sync.Mutex{},
	}
}

func (t *Template) Render(w io.Writer, name string, data any, c echo.Context) error {
	t.lock.Lock()
	if t.reload {
		t.templates = template.Must(template.ParseGlob("html/*.html"))
	}
	t.lock.Unlock()
	return t.templates.ExecuteTemplate(w, name, data)
}
