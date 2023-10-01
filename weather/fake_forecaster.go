package weather

import (
	"sync"
	"time"
)

type FakeForecaster struct {
	forecast Forecast
	err      error
	lock     *sync.Mutex
}

func NewFakeForecaster() *FakeForecaster {
	return &FakeForecaster{
		lock: &sync.Mutex{},
	}
}

func (f *FakeForecaster) ForecastFor(t time.Time) (Forecast, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.forecast.StartTime = t
	f.forecast.EndTime = t.Add(1 * time.Hour)

	return f.forecast, f.err
}

func (f *FakeForecaster) SetForecast(forecast Forecast) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.forecast = forecast
}

func (f *FakeForecaster) SetError(err error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.err = err
}
