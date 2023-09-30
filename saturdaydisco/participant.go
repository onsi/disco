package saturdaydisco

import (
	"fmt"
	"strings"

	"github.com/onsi/disco/mail"
	"github.com/onsi/say"
)

type Participant struct {
	Address        mail.EmailAddress
	Count          int
	RelevantEmails []mail.Email
}

func (p Participant) dup() Participant {
	emails := []mail.Email{}
	return Participant{
		Address:        p.Address,
		Count:          p.Count,
		RelevantEmails: append(emails, p.RelevantEmails...),
	}
}

func (p Participant) IndentedRelevantEmails() string {
	out := &strings.Builder{}
	for idx, email := range p.RelevantEmails {
		say.Fpiw(out, 2, 100, "%s\n", email.String())
		if idx < len(p.RelevantEmails)-1 {
			say.Fpi(out, 2, "---\n")
		}
	}
	return out.String()
}

type Participants []Participant

func (p Participants) UpdateCount(address mail.EmailAddress, count int, relevantEmail mail.Email) Participants {
	for i := range p {
		if p[i].Address.Equals(address) {
			if !p[i].Address.HasExplicitName() {
				p[i].Address = address
			}
			p[i].Count = count
			p[i].RelevantEmails = append(p[i].RelevantEmails, relevantEmail)
			return p
		}
	}
	return append(p, Participant{
		Address:        address,
		Count:          count,
		RelevantEmails: []mail.Email{relevantEmail},
	})
}

func (p Participants) CountFor(address mail.EmailAddress) int {
	for _, participant := range p {
		if participant.Address.Equals(address) {
			return participant.Count
		}
	}
	return 0
}

func (p Participants) Count() int {
	total := 0
	for _, participant := range p {
		total += participant.Count
	}
	return total
}

func (p Participants) Public() string {
	if p.Count() == 0 {
		return "No one's signed up yet"
	}

	validP := Participants{}
	for _, participant := range p {
		if participant.Count > 0 {
			validP = append(validP, participant)
		}
	}
	out := &strings.Builder{}
	for i, participant := range validP {
		out.WriteString(participant.Address.Name())
		if participant.Count > 1 {
			fmt.Fprintf(out, " **(%d)**", participant.Count)
		}
		if i < len(validP)-2 {
			out.WriteString(", ")
		} else if i == len(validP)-2 {
			out.WriteString(" and ")
		}
	}
	return out.String()
}

func (p Participants) dup() Participants {
	participants := make(Participants, len(p))
	for i, participant := range p {
		participants[i] = participant.dup()
	}
	return participants
}
