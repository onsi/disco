package main

import (
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/mail"
)

type Server struct {
	e      *echo.Echo
	config config.Config

	TempEmails []string
}

func main() {
	server := &Server{
		e:      echo.New(),
		config: config.LoadConfig(),
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
	email, err := mail.ParseIncomingEmail(data)
	if err != nil {
		s.e.Logger.Errorf("failed to parse incoming email: %s", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}

	go func() {
		if strings.HasPrefix(email.Text, "/reply-all") {
			mail.SendEmail(email.ReplyAll("saturday-disco@sedenverultimate.net", "Got **your** message!\n\n_Thanks!_\n\n- Disco 🪩"))
		} else if strings.HasPrefix(email.Text, "/reply") {
			mail.SendEmail(email.Reply("saturday-disco@sedenverultimate.net", "Got **your** message!\n\n_Thanks!_\n\n- Disco 🪩"))
		} else {
			mail.SendEmail(mail.Email{
				From:    "saturday-disco@sedenverultimate.net",
				To:      []mail.EmailAddress{email.From},
				Subject: "Got your message",
				Text:    string(data),
			})
		}
	}()
	return c.NoContent(http.StatusOK)
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}
