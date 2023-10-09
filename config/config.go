package config

import (
	"os"

	"github.com/onsi/disco/mail"
)

type Config struct {
	BossEmail           mail.EmailAddress
	SaturdayDiscoEmail  mail.EmailAddress
	SaturdayDiscoList   mail.EmailAddress
	LunchtimeDiscoEmail mail.EmailAddress
	LunchtimeDiscoList  mail.EmailAddress

	Port                       string
	Env                        string
	ForwardEmailKey            string
	IncomingSaturdayEmailGUID  string
	IncomingLunchtimeEmailGUID string
	OpenAIKey                  string

	AWSAccessKey string
	AWSSecretKey string
	AWSRegion    string
	AWSS3Bucket  string
}

func (c Config) IsPROD() bool {
	return c.Env == "PROD"
}

func (c Config) IsDev() bool {
	return !c.IsPROD()
}

func LoadConfig() Config {
	return Config{
		Port:                       os.Getenv("PORT"),
		Env:                        os.Getenv("ENV"),
		ForwardEmailKey:            os.Getenv("FORWARD_EMAIL_KEY"),
		IncomingSaturdayEmailGUID:  os.Getenv("INCOMING_SATURDAY_EMAIL_GUID"),
		IncomingLunchtimeEmailGUID: os.Getenv("INCOMING_LUNCHTIME_EMAIL_GUID"),
		OpenAIKey:                  os.Getenv("OPEN_AI_KEY"),
		AWSAccessKey:               os.Getenv("AWS_ACCESS_KEY"),
		AWSSecretKey:               os.Getenv("AWS_SECRET_KEY"),
		AWSRegion:                  os.Getenv("AWS_REGION"),
		AWSS3Bucket:                os.Getenv("AWS_S3_BUCKET"),

		BossEmail:           mail.EmailAddress(os.Getenv("BOSS_EMAIL")),
		SaturdayDiscoEmail:  mail.EmailAddress(os.Getenv("SATURDAY_DISCO_EMAIL")),
		SaturdayDiscoList:   mail.EmailAddress(os.Getenv("SATURDAY_DISCO_LIST")),
		LunchtimeDiscoEmail: mail.EmailAddress(os.Getenv("LUNCHTIME_DISCO_EMAIL")),
		LunchtimeDiscoList:  mail.EmailAddress(os.Getenv("LUNCHTIME_DISCO_LIST")),
	}
}
