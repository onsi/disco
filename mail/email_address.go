package mail

import "strings"

type EmailAddresses []EmailAddress

func (e EmailAddresses) String() string {
	addresses := []string{}
	for _, address := range e {
		addresses = append(addresses, address.String())
	}
	return strings.Join(addresses, ", ")
}

type EmailAddress string

func (e EmailAddress) String() string {
	return strings.TrimSpace(string(e))
}

func (e EmailAddress) HasExplicitName() bool {
	tidy := e.String()
	return strings.LastIndex(tidy, " ") != -1
}

func (e EmailAddress) Name() string {
	tidy := e.String()
	if strings.LastIndex(tidy, " ") == -1 {
		return strings.Split(e.Address(), "@")[0]
	}

	return strings.Title(strings.Trim(strings.Split(tidy, " ")[0], " "))
}

func (e EmailAddress) Address() string {
	tidy := e.String()
	addressPortion := tidy[strings.LastIndex(tidy, " ")+1:]
	return strings.Trim(addressPortion, "<>")
}

func (e EmailAddress) Equals(other EmailAddress) bool {
	return strings.ToLower(e.Address()) == strings.ToLower(other.Address())
}
