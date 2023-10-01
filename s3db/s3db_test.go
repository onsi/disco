package s3db_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/s3db"
)

type ObjectToStore struct {
	Content string
	Time    time.Time
}

var _ = Describe("S3db", func() {
	var db s3db.S3DBInt
	BeforeEach(func() {
		var err error
		db, err = s3db.NewS3DB()
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("roundtrips successfully", func() {
		obj := ObjectToStore{
			Content: "save me please",
			Time:    time.Now().In(time.Local),
		}
		data, err := json.Marshal(obj)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(db.PutObject("test-key", data)).Should(Succeed())

		var retrieved ObjectToStore
		data, err = db.FetchObject("test-key")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(json.Unmarshal(data, &retrieved)).Should(Succeed())
		Ω(retrieved).Should(Equal(obj))
	})

	It("fails if the object is not found", func() {
		data, err := db.FetchObject("bloop")
		Ω(err).Should(MatchError(s3db.ErrObjectNotFound))
		Ω(data).Should(BeEmpty())
	})
})
