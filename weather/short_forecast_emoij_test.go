package weather_test

import (
	"encoding/json"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/disco/s3db"
	"github.com/onsi/disco/weather"
)

var _ = Describe("ShortForecastEmoij", func() {
	It("returns appropriate emoji", SpecTimeout(time.Second*20), func(ctx SpecContext) {
		if os.Getenv("INCLUDE_LLM_SPECS") != "true" {
			Skip("Skipping LLM specs - use INCLUDE_LLM_SPECS=true to run them")
		}

		db := s3db.NewFakeS3DB()

		cache := map[string]string{
			"the cache is used": "😎",
		}
		data, err := json.Marshal(cache)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(db.PutObject(weather.KEY, data)).Should(Succeed())

		provider := weather.NewShortForecastEmojiProvider(db)
		Ω(provider.GetShortForecastEmoji(ctx, "Sunny")).Should(Equal("☀️"))
		Ω(provider.GetShortForecastEmoji(ctx, "Rainy")).Should(Equal("🌧️"))
		Ω(provider.GetShortForecastEmoji(ctx, "Snowy")).Should(Equal("❄️"))
		Ω(provider.GetShortForecastEmoji(ctx, "The Cache is Used")).Should(Equal("😎"))

		cache = map[string]string{}
		data, err = db.FetchObject(weather.KEY)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(json.Unmarshal(data, &cache)).Should(Succeed())
		Ω(cache).Should(Equal(map[string]string{
			"sunny":             "☀️",
			"rainy":             "🌧️",
			"snowy":             "❄️",
			"the cache is used": "😎",
		}))
	})
})
