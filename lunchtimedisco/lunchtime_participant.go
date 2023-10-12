package lunchtimedisco

import (
	"strings"

	"github.com/onsi/disco/mail"
)

type LunchtimeParticipant struct {
	Address  mail.EmailAddress `json:"address"`
	GameKeys []string          `json:"gameKeys"`
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

func (ps LunchtimeParticipants) AddOrUpdate(participant LunchtimeParticipant) LunchtimeParticipants {
	// remove if need be
	if participant.GameKeys == nil || len(participant.GameKeys) == 0 {
		out := LunchtimeParticipants{}
		for i := range ps {
			if !ps[i].Address.Equals(participant.Address) {
				out = append(out, ps[i])
			}
		}
		return out
	}
	// update if present
	for i := range ps {
		if ps[i].Address.Equals(participant.Address) {
			ps[i] = participant
			return ps
		}
	}
	// otherwise add
	return append(ps, participant)
}

type HistoricalParticipants mail.EmailAddresses

func (p HistoricalParticipants) AddOrUpdate(address mail.EmailAddress) HistoricalParticipants {
	for i := range p {
		if p[i].Equals(address) {
			p[i] = address
			return p
		}
	}
	return append(p, address)
}
