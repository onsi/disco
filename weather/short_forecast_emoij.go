package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/onsi/disco/askgpt"
	"github.com/onsi/disco/s3db"
)

const KEY = "EmojiCache"

type ShortForecastEmojiProvider struct {
	cache map[string]string
	ready bool
	db    s3db.S3DBInt
	lock  *sync.Mutex
}

func NewShortForecastEmojiProvider(db s3db.S3DBInt) *ShortForecastEmojiProvider {
	return &ShortForecastEmojiProvider{
		cache: make(map[string]string),
		ready: false,
		db:    db,
		lock:  &sync.Mutex{},
	}
}

func (p *ShortForecastEmojiProvider) getReady() {
	if p.ready {
		return
	}
	p.ready = true
	data, err := p.db.FetchObject(KEY)
	if err != nil {
		if err == s3db.ErrObjectNotFound {
			//we just haven't warmed up the cache yet.  That's ok
			return
		}
		fmt.Println("Error fetching emoji cache", err)
		return
	}
	cache := map[string]string{}
	err = json.Unmarshal(data, &cache)
	if err != nil {
		fmt.Println("Error unmarshaling emoji cache", err)
		return
	}
	p.cache = cache
}

func (p *ShortForecastEmojiProvider) saveCache() {
	data, err := json.Marshal(p.cache)
	if err != nil {
		fmt.Println("Error saving emoji cache", err)
		return
	}
	p.db.PutObject(KEY, data)
}

func (p *ShortForecastEmojiProvider) GetShortForecastEmoji(ctx context.Context, forecast string) string {
	p.lock.Lock()
	defer p.lock.Unlock()
	forecast = strings.TrimSpace(strings.ToLower(forecast))

	p.getReady()

	if emoji, ok := p.cache[forecast]; ok {
		return emoji
	}

	emoji, err := askgpt.AskGPT3(ctx, "Give me a single emoji from this set: ☀️🌤️⛅️🌥️☁️🌦️🌧️⛈️🌩️🌨️❄️💨 that best characterizes this short weather forecast", forecast)
	if err != nil {
		fmt.Printf("Forecast %s returned error while getting emoji: %s\n", forecast, err.Error())
		return ""
	}
	if emoji == "" {
		fmt.Printf("Forecast %s returned empty emoji\n", forecast)
		return ""
	}

	p.cache[forecast] = emoji
	p.saveCache()
	return emoji
}
