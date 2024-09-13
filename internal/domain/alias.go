package domain

import (
	"net/url"
	"time"
)

type TTLParams struct {
	TriesLeft   int
	IsPermanent bool
}

// Alias is a struct that represents an alias for an origin url.
type Alias struct {
	ID       string
	Key      string
	URL      *url.URL
	IsActive bool
	Params   TTLParams
}

// AliasCreationRequest is a struct that represents an alias creation request.
type AliasCreationRequest struct {
	Params TTLParams
	URL    *url.URL
}

func (a Alias) Type() string {

	if a.Params.IsPermanent {
		return "permanent"
	}
	return "ttl-restricted"
}

// Redirected is a function that creates an AliasLinkRedirected event.
func (a Alias) Redirected() AliasUsed {
	return AliasUsed{
		Alias:      a,
		OccurredAt: time.Now(),
	}
}

// Expired is a function that creates an URLExpired event.
func (a Alias) Expired() AliasExpired {
	return AliasExpired{
		Alias:      a,
		OccurredAt: time.Now(),
	}
}
