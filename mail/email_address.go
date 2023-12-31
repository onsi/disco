package mail

import "strings"

type EmailAddresses []EmailAddress

func (e EmailAddresses) dup() EmailAddresses {
	out := make(EmailAddresses, len(e))
	copy(out, e)
	return out
}

func (e EmailAddresses) String() string {
	addresses := []string{}
	for _, address := range e {
		addresses = append(addresses, address.String())
	}
	return strings.Join(addresses, ", ")
}

func (e EmailAddresses) Strings() []string {
	out := []string{}
	for _, address := range e {
		out = append(out, address.String())
	}
	return out
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

	commaIdx := strings.Index(tidy, ",")
	if commaIdx != -1 {
		tidy = strings.Trim(tidy[commaIdx:], ", ")
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
