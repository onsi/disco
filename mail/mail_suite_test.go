package mail_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestMail(t *testing.T) {
	os.Setenv("ENV", "TEST")
	format.TruncatedDiff = false
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mail Suite")
}
