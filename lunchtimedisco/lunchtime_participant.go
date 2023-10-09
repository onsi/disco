package lunchtimedisco

import (
	"fmt"
	"strings"

	"github.com/onsi/disco/mail"
)

var AllGameKeys = map[string]bool{
	"ALL": true,
	"YES": true,
}

var ClearGameKeys = map[string]bool{
	"CLEAR": true,
	"NONE":  true,
	"NO":    true,
	"0":     true,
}

type LunchtimeParticipant struct {
	Address  mail.EmailAddress
	GameKeys []string
}

func (p LunchtimeParticipant) dup() LunchtimeParticipant {
	gameKeys := []string{}
	return LunchtimeParticipant{
		Address:  p.Address,
		GameKeys: append(gameKeys, p.GameKeys...),
	}
}

type LunchtimeParticipants []LunchtimeParticipant

func (p LunchtimeParticipants) dup() LunchtimeParticipants {
	out := LunchtimeParticipants{}
	for _, participant := range p {
		out = append(out, participant.dup())
	}
	return out
}

func (p LunchtimeParticipants) GamesFor(address mail.EmailAddress) string {
	for _, participant := range p {
		if participant.Address.Equals(address) {
			return strings.Join(participant.GameKeys, ",")
		}
	}
	return ""
}

func (p LunchtimeParticipants) parseGameKeyRange(input string) ([]string, bool) {
	negated := false
	input = strings.TrimSpace(input)
	if AllGameKeys[input] {
		return GameKeys, negated
	}
	if strings.HasPrefix(input, "!") {
		negated = true
		input = strings.TrimSpace(input[1:])
	}
	if ValidGameKeys[input] {
		return []string{input}, negated
	}
	components := strings.Split(input, "-")
	if len(components) != 2 {
		return nil, false
	}
	start := strings.TrimSpace(components[0])
	end := strings.TrimSpace(components[1])
	if !(ValidGameKeys[start] && ValidGameKeys[end]) {
		return nil, false
	}
	startIdx, endIdx := -1, -1
	for idx, key := range GameKeys {
		if key == start {
			startIdx = idx
		}
		if key == end {
			endIdx = idx
		}
	}
	if startIdx > endIdx {
		return nil, false
	}
	out := []string{}
	for idx := startIdx; idx <= endIdx; idx++ {
		out = append(out, GameKeys[idx])
	}
	return out, negated
}

func (p LunchtimeParticipants) UpdateGameKeys(address mail.EmailAddress, input string) (LunchtimeParticipants, string, error) {
	input = strings.ToUpper(strings.TrimSpace(input))
	if ClearGameKeys[input] {
		for i := range p {
			if p[i].Address.Equals(address) {
				return append(p[:i], p[i+1:]...), "cleared", nil
			}
		}
		return p, "(nothing to clear)", nil
	}

	preliminaryGameKeys := []string{}
	negatedGameKeys := map[string]bool{}
	seen := map[string]bool{}
	for _, component := range strings.Split(input, ",") {
		parsedGameKeys, negated := p.parseGameKeyRange(component)
		if parsedGameKeys == nil {
			return nil, "", fmt.Errorf("invalid input: %s - %s is not a valid game-key.  Must be a comma separated list of single-letters or ranges (e.g. A,B-D,E).  Negation is supported (e.g. A-J,!C,!E-F)", input, component)
		}
		for _, gameKey := range parsedGameKeys {
			if negated {
				negatedGameKeys[gameKey] = true
			} else if !seen[gameKey] {
				preliminaryGameKeys = append(preliminaryGameKeys, gameKey)
				seen[gameKey] = true
			}
		}
	}

	gameKeys := []string{}
	for _, gameKey := range preliminaryGameKeys {
		if !negatedGameKeys[gameKey] {
			gameKeys = append(gameKeys, gameKey)
		}
	}

	if len(gameKeys) == 0 {
		return nil, "", fmt.Errorf("invalid input: %s - no game-keys were left after processing", input)
	}

	for i := range p {
		if p[i].Address.Equals(address) {
			if !p[i].Address.HasExplicitName() {
				p[i].Address = address
			}
			p[i].GameKeys = gameKeys
			return p, "updated to " + strings.Join(gameKeys, ","), nil
		}
	}
	return append(p, LunchtimeParticipant{
		Address:  address,
		GameKeys: gameKeys,
	}), "set to " + strings.Join(gameKeys, ","), nil
}
