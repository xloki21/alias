package domain

import (
	"net/url"
	"time"
)

// Alias is a struct that represents an alias for an origin url.
type Alias struct {
	ID          string
	Origin      *url.URL
	URL         *url.URL
	TTL         int
	IsActive    bool
	IsPermanent bool
}

func (a Alias) Type() string {
	if a.IsPermanent {
		return "permanent"
	}
	return "ttl-restricted"
}

// Redirected is a function that creates an AliasLinkRedirected event.
func (a Alias) Redirected() AliasLinkRedirected {
	return AliasLinkRedirected{
		Alias:      a,
		OccurredAt: time.Now(),
	}
}

// Expired is a function that creates an URLExpired event.
func (a Alias) Expired() URLExpired {
	return URLExpired{
		Alias:      a,
		OccurredAt: time.Now(),
	}
}
