package server

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/onsi/disco/config"
	"github.com/onsi/disco/lunchtimedisco"
	"github.com/onsi/disco/mail"
	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/saturdaydisco"
)

type TemplateData struct {
	Saturday  saturdaydisco.TemplateData
	Lunchtime lunchtimedisco.TemplateData
}

type Server struct {
	e              *echo.Echo
	config         config.Config
	outbox         mail.OutboxInt
	db             s3db.S3DBInt
	saturdayDisco  *saturdaydisco.SaturdayDisco
	lunchtimeDisco *lunchtimedisco.LunchtimeDisco
}

func NewServer(e *echo.Echo, conf config.Config, outbox mail.OutboxInt, db s3db.S3DBInt, saturdayDisco *saturdaydisco.SaturdayDisco, lunchtimeDisco *lunchtimedisco.LunchtimeDisco) *Server {
	return &Server{
		e:              e,
		config:         conf,
		outbox:         outbox,
		db:             db,
		saturdayDisco:  saturdayDisco,
		lunchtimeDisco: lunchtimeDisco,
	}
}

func (s *Server) Start() error {
	t := NewTemplateRenderer(s.config.IsDev())
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
	s.e.Static("/img", "img")
	s.e.GET("/", s.Index)
	s.e.POST("/incoming/"+s.config.IncomingSaturdayEmailGUID, s.IncomingSaturdayEmail)
	s.e.POST("/incoming/"+s.config.IncomingLunchtimeEmailGUID, s.IncomingLunchtimeEmail)
	s.e.POST("/subscribe", s.Subscribe)
	s.e.GET("/lunchtime/:guid", s.Lunchtime)
	s.e.POST("/lunchtime/:guid", s.LunchtimeSubmit)
}

func (s *Server) Index(c echo.Context) error {
	return c.Render(http.StatusOK, "index", TemplateData{
		Saturday:  s.saturdayDisco.TemplateData(),
		Lunchtime: s.lunchtimeDisco.TemplateData(),
	})
}

func (s *Server) IncomingSaturdayEmail(c echo.Context) error {
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

func (s *Server) IncomingLunchtimeEmail(c echo.Context) error {
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	email, err := mail.ParseIncomingEmail(s.db, data, s.e.Logger.Output())
	if err != nil {
		s.e.Logger.Errorf("failed to parse incoming email: %s", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	s.lunchtimeDisco.HandleIncomingEmail(email)
	return c.NoContent(http.StatusOK)
}

func (s *Server) Lunchtime(c echo.Context) error {
	data := s.lunchtimeDisco.TemplateData()
	guid := c.Param("guid")
	if guid == data.GUID {
		return c.Render(http.StatusOK, "lunchtime_player", TemplateData{
			Lunchtime: data,
		})
	} else if guid == data.BossGUID {
		return c.Render(http.StatusOK, "lunchtime_boss", TemplateData{
			Lunchtime: data,
		})
	}
	return c.String(http.StatusNotFound, "not found - check your inbox for the latest game link")
}

func (s *Server) LunchtimeSubmit(c echo.Context) error {
	data := s.lunchtimeDisco.TemplateData()
	guid := c.Param("guid")
	if guid == data.GUID {
		var participant lunchtimedisco.LunchtimeParticipant
		if err := c.Bind(&participant); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		s.lunchtimeDisco.HandleParticipant(participant)
		return c.NoContent(http.StatusOK)
	} else if guid == data.BossGUID {
		var command lunchtimedisco.Command
		if err := c.Bind(&command); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		s.lunchtimeDisco.HandleCommand(command)
		return c.NoContent(http.StatusOK)
	}
	return c.String(http.StatusUnauthorized, "not allowed")
}
