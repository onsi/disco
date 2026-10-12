package server_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestServer(t *testing.T) {
	os.Setenv("ENV", "TEST")
	format.TruncatedDiff = false
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}
