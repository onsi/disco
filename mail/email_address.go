package mail

import "strings"

type EmailAddress string

func (e EmailAddress) String() string {
	return strings.TrimSpace(string(e))
}

func (e EmailAddress) Name() string {
	tidy := e.String()
	if strings.LastIndex(tidy, " ") == -1 {
		return strings.Split(e.Address(), "@")[0]
	}
	return strings.Trim(strings.Split(tidy, " ")[0], "<>")
}

func (e EmailAddress) Address() string {
	tidy := e.String()
	addressPortion := tidy[strings.LastIndex(tidy, " ")+1:]
	return strings.Trim(addressPortion, "<>")
}

func (e EmailAddress) Equals(other EmailAddress) bool {
	return e.Address() == other.Address()
}
