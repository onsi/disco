package lunchtimedisco

import "github.com/onsi/disco/mail"

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
