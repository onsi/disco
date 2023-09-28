package config

import (
	"os"

	"github.com/onsi/disco/mail"
)

type Config struct {
	BossEmail          mail.EmailAddress
	SaturdayDiscoEmail mail.EmailAddress
	SaturdayDiscoList  mail.EmailAddress

	Port              string
	Env               string
	ForwardEmailKey   string
	IncomingEmailGUID string
}

func (c Config) IsPROD() bool {
	return c.Env == "PROD"
}

func (c Config) IsDev() bool {
	return !c.IsPROD()
}

func LoadConfig() Config {
	return Config{
		Port:              os.Getenv("PORT"),
		Env:               os.Getenv("ENV"),
		ForwardEmailKey:   os.Getenv("FORWARD_EMAIL_KEY"),
		IncomingEmailGUID: os.Getenv("INCOMING_EMAIL_GUID"),
	}
}
