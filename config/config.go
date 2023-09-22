package config

import "os"

type Config struct {
	Port string
	Env  string
}

func (c Config) IsPROD() bool {
	return c.Env == "PROD"
}

func (c Config) IsDev() bool {
	return !c.IsPROD()
}

func LoadConfig() Config {
	return Config{
		Port: os.Getenv("PORT"),
		Env:  os.Getenv("ENV"),
	}
}
