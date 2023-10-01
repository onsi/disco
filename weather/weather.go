package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/onsi/disco/s3db"
)

//set up admin@sedevnerultimate.net in forwardemail and as the user-agent
//http://www.weather.gov/documentation/services-web-api
// why am i not gettin us units?

const DEFAULT_TIMEOUT = 10 * time.Second
const API_ENDPOINT = "https://api.weather.gov"
const USER_AGENT = "(www.sedenverultimate.net, admin@sedenverultimate.net)"
const FETCH_FREQUENCY = 6 * time.Hour

var JamesBibleParkLatLong = []string{"39.6656062", "-104.9071077"}

type Forecast struct {
	StartTime                      time.Time `json:"startTime"`
	EndTime                        time.Time `json:"endTime"`
	Temperature                    int       `json:"temperature"`
	TemperatureUnit                string    `json:"temperatureUnit"`
	ProbabilityOfPrecipitation     int
	ProbabilityOfPrecipitationJSON struct {
		Value int `json:"value"`
	} `json:"probabilityOfPrecipitation"`
	WindSpeed          string `json:"windSpeed"`
	ShortForecast      string `json:"shortForecast"`
	ShortForecastEmoji string
}

func (f Forecast) IsZero() bool {
	return f.StartTime.IsZero() && f.EndTime.IsZero()
}

func (f Forecast) TemperatureEmoji() string {
	if f.Temperature < 45 {
		return "🥶"
	} else if f.Temperature > 80 {
		return "🥵"
	} else {
		return "😎"
	}
}

func (f Forecast) String() string {
	if f.IsZero() {
		return "Weather forecast is unavailable"
	}
	out := &strings.Builder{}
	if f.ShortForecastEmoji != "" {
		out.WriteString(f.ShortForecastEmoji)
		out.WriteString(" ")
	}
	fmt.Fprintf(out, "%s: %s %dº%s | 💧 %d%% | 💨 %s", f.ShortForecast, f.TemperatureEmoji(), f.Temperature, f.TemperatureUnit, f.ProbabilityOfPrecipitation, f.WindSpeed)
	return out.String()
}

type ForecasterInt interface {
	ForecastFor(t time.Time) (Forecast, error)
}

type Forecaster struct {
	db                         s3db.S3DBInt
	shortForecastEmojiProvider *ShortForecastEmojiProvider
	lastFetched                time.Time
	cachedForecasts            []Forecast
	lock                       *sync.Mutex
}

func NewForecaster(db s3db.S3DBInt) *Forecaster {
	return &Forecaster{
		db:                         db,
		shortForecastEmojiProvider: NewShortForecastEmojiProvider(db),
		lastFetched:                time.Time{},
		cachedForecasts:            []Forecast{},
		lock:                       &sync.Mutex{},
	}
}

func (f *Forecaster) ForecastFor(t time.Time) (Forecast, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), DEFAULT_TIMEOUT)
	defer cancel()

	if f.lastFetched.IsZero() || time.Since(f.lastFetched) > FETCH_FREQUENCY {
		forecasts, err := f.getForecasts(ctx)
		if err != nil {
			return Forecast{}, err
		}
		f.cachedForecasts = forecasts
		f.lastFetched = time.Now()
	}

	winner := Forecast{}
	for _, forecast := range f.cachedForecasts {
		if !t.Before(forecast.StartTime) && t.Before(forecast.EndTime) {
			winner = forecast
			break
		}
	}

	if winner.IsZero() {
		return Forecast{}, fmt.Errorf("no forecast found for time %v", t)
	}
	winner.ProbabilityOfPrecipitation = winner.ProbabilityOfPrecipitationJSON.Value
	winner.ShortForecastEmoji = f.shortForecastEmojiProvider.GetShortForecastEmoji(ctx, winner.ShortForecast)

	return winner, nil
}

func (f *Forecaster) getForecasts(ctx context.Context) ([]Forecast, error) {
	type pointsResponseStruct struct {
		Properties struct {
			ForecastHourly string `json:"forecastHourly"`
		} `json:"properties"`
	}

	type forecastHourlyResponseStruct struct {
		Properties struct {
			Periods []Forecast `json:"periods"`
		} `json:"properties"`
	}

	req, err := http.NewRequestWithContext(ctx, "GET", API_ENDPOINT+"/points/"+JamesBibleParkLatLong[0]+","+JamesBibleParkLatLong[1], nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate points request: %w", err)
	}
	req.Header.Add("User-Agent", USER_AGENT)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make points request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to make points request: got %d", resp.StatusCode)
	}

	var pointsResponse pointsResponseStruct
	err = json.NewDecoder(resp.Body).Decode(&pointsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse points response: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, "GET", pointsResponse.Properties.ForecastHourly, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate forecast request: %w", err)
	}
	req.Header.Add("User-Agent", USER_AGENT)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make forecast request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to make forecast request: got %d", resp.StatusCode)
	}

	var forecastHourlyResponse forecastHourlyResponseStruct
	err = json.NewDecoder(resp.Body).Decode(&forecastHourlyResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse forecast response: %w", err)
	}

	return forecastHourlyResponse.Properties.Periods, nil
}
