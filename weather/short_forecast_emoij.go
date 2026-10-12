package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/onsi/disco/askclaude"
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

	emoji, err := askForEmoji(ctx, forecast)
	if err != nil {
		fmt.Printf("Forecast %s returned error while getting emoji: %s\n", forecast, err.Error())
		return ""
	}

	p.cache[forecast] = emoji
	p.saveCache()
	return emoji
}

var forecastEmoji = []string{"☀️", "🌤️", "⛅️", "🌥️", "☁️", "🌦️", "🌧️", "⛈️", "🌩️", "🌨️", "❄️", "💨"}

const emojiPrompt = `You pick the emoji that summarizes a short weather forecast from api.weather.gov in the email for an outdoor pickup ultimate frisbee game. The forecast is inside <forecast> tags.

Choose exactly one emoji from this set:
- ☀️ sunny or clear
- 🌤️ mostly sunny or mostly clear
- ⛅️ partly sunny or partly cloudy
- 🌥️ mostly cloudy
- ☁️ cloudy, overcast, or fog
- 🌦️ a chance of showers, or showers with some sun
- 🌧️ rain, drizzle, or showers likely
- ⛈️ thunderstorms
- 🌩️ lightning, or a slight chance of thunderstorms
- 🌨️ snow showers, flurries, sleet, or a wintry mix
- ❄️ snow
- 💨 windy, breezy, or blowing dust

When the forecast changes during the day ("Sunny then Chance Showers"), pick the condition that matters most for playing outside.`

var emojiSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"emoji": map[string]any{
			"type": "string",
			"enum": forecastEmoji,
		},
	},
	"required":             []string{"emoji"},
	"additionalProperties": false,
}

func askForEmoji(ctx context.Context, forecast string) (string, error) {
	var response struct {
		Emoji string `json:"emoji"`
	}
	err := askclaude.AskForJSON(ctx, emojiPrompt, "<forecast>"+forecast+"</forecast>", emojiSchema, &response)
	if err != nil {
		return "", err
	}
	return canonicalForecastEmoji(response.Emoji)
}

func canonicalForecastEmoji(emoji string) (string, error) {
	bare := withoutVariationSelector(strings.TrimSpace(emoji))
	for _, candidate := range forecastEmoji {
		if withoutVariationSelector(candidate) == bare {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%q is not a forecast emoji", emoji)
}

func withoutVariationSelector(s string) string {
	return strings.ReplaceAll(s, "\uFE0F", "")
}
