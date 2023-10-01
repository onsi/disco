package weather_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWeather(t *testing.T) {
	os.Setenv("ENV", "TEST")
	RegisterFailHandler(Fail)
	RunSpecs(t, "Weather Suite")
}
