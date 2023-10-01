package main

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/saturdaydisco"
	"github.com/onsi/disco/weather"
)

type Server struct {
	e             *echo.Echo
	config        config.Config
	outbox        mail.OutboxInt
	saturdayDisco *saturdaydisco.SaturdayDisco

	TempEmails []string
}

func main() {
	conf := config.LoadConfig()
	server := &Server{
		e:      echo.New(),
		config: conf,
		outbox: mail.NewOutbox(conf.ForwardEmailKey),
	}
	log.Fatal(server.Start())
}

func (s *Server) Start() error {
	t := &Template{
		templates: template.Must(template.ParseGlob("html/*.html")),
	}
	s.e.Renderer = t
	s.e.Logger.SetLevel(log.INFO)
	if s.config.IsDev() {
		s.e.Debug = true
	}

	db, err := s3db.NewS3DB()
	if err != nil {
		return err
	}
	saturdaydisco, err := saturdaydisco.NewSaturdayDisco(
		s.config,
		s.e.Logger.Output(),
		saturdaydisco.NewAlarmClock(),
		mail.NewOutbox(s.config.ForwardEmailKey),
		saturdaydisco.NewInterpreter(),
		weather.NewForecaster(db),
		db,
	)
	if err != nil {
		return err
	}
	s.saturdayDisco = saturdaydisco
	s.RegisterRoutes()
	return s.e.Start(":" + s.config.Port)
}

func (s *Server) RegisterRoutes() {
	s.e.Use(middleware.Logger())
	s.e.GET("/", s.Index)
	s.e.POST("/incoming/"+s.config.IncomingEmailGUID, s.IncomingEmail)
}

func (s *Server) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index", s)
}

func (s *Server) IncomingEmail(c echo.Context) error {
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	email, err := mail.ParseIncomingEmail(data, s.e.Logger.Output())
	if err != nil {
		s.e.Logger.Errorf("failed to parse incoming email: %s", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}

	s.saturdayDisco.HandleIncomingEmail(email)
	return c.NoContent(http.StatusOK)
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}
