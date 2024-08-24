package domain

import (
	"fmt"
	"time"
)

// AliasLinkRedirected is a struct that represents an alias link redirect event.
type AliasLinkRedirected struct {
	Alias
	OccurredAt time.Time
}

func (a AliasLinkRedirected) String() string {
	return fmt.Sprintf("%s: %s", "AliasLinkRedirected", a.URL)
}

type URLExpired struct {
	Alias
	OccurredAt time.Time
}

func (u URLExpired) String() string {
	return fmt.Sprintf("%s: %s", "URLExpired", u.URL)
}
