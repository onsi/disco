package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/config"
)

type Server struct {
	e      *echo.Echo
	config config.Config
}

func main() {
	fmt.Println(os.Environ())
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
	fmt.Println(s.config)
	return s.e.Start(":" + s.config.Port)
}

func (s *Server) RegisterRoutes() {
	s.e.Use(middleware.Logger())
	s.e.GET("/", s.Index)
}

func (s *Server) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index", nil)
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}
