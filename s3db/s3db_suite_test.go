package s3db_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestS3db(t *testing.T) {
	os.Setenv("ENV", "TEST")
	RegisterFailHandler(Fail)
	RunSpecs(t, "S3db Suite")
}
