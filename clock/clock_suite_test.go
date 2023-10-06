package clock_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clock Suite")
}
