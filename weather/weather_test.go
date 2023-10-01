package weather_test

import (
	"time"

	"github.com/onsi/disco/s3db"
	. "github.com/onsi/disco/weather"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Weather forecasts", func() {
	Describe("Forecasts", func() {
		It("has a temperature emoji", func() {
			Ω(Forecast{Temperature: 30}.TemperatureEmoji()).Should(Equal("🥶"))
			Ω(Forecast{Temperature: 81}.TemperatureEmoji()).Should(Equal("🥵"))
			Ω(Forecast{Temperature: 60}.TemperatureEmoji()).Should(Equal("😎"))
		})

		It("stringifies nicely", func() {
			Ω(Forecast{
				StartTime:                  time.Now(),
				EndTime:                    time.Now().Add(time.Hour),
				Temperature:                72,
				TemperatureUnit:            "F",
				WindSpeed:                  "8 mph",
				ProbabilityOfPrecipitation: 10,
				ShortForecast:              "Partly Cloud",
				ShortForecastEmoji:         "🌤️",
			}.String()).Should(Equal("🌤️ Partly Cloud: 😎 72ºF | 💧 10% | 💨 8 mph"))

			Ω(Forecast{}.String()).Should(Equal("Weather forecast is unavailable"))
		})
	})

	It("works", func() {
		db := s3db.NewFakeS3DB()
		forecaster := NewForecaster(db)

		referenceTime := time.Now().Add(24 * time.Hour)

		t := time.Now()
		forecast, err := forecaster.ForecastFor(referenceTime)
		firstHit := time.Since(t)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(forecast).ShouldNot(BeZero())
		Ω(forecast.StartTime).Should(BeTemporally("~", referenceTime, time.Hour))
		Ω(forecast.ShortForecast).ShouldNot(BeZero())
		Ω(forecast.ShortForecastEmoji).ShouldNot(BeZero())

		t = time.Now()
		cachedForecast, err := forecaster.ForecastFor(referenceTime)
		Ω(err).ShouldNot(HaveOccurred())
		cacheHit := time.Since(t)

		Ω(firstHit).Should(BeNumerically(">", time.Millisecond*10))
		Ω(cacheHit).Should(BeNumerically("<", time.Millisecond*10))
		Ω(cachedForecast).Should(Equal(forecast))
	})
})
