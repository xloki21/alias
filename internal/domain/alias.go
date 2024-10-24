package domain

import (
	"github.com/google/uuid"
	"net/url"
	"time"
)

type TTLParams struct {
	TriesLeft   uint64
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

// CreateRequest is a struct that represents an alias creation request.
type CreateRequest struct {
	Params TTLParams
	URL    *url.URL
}

func (a Alias) Type() string {

	if a.Params.IsPermanent {
		return "permanent"
	}
	return "ttl-restricted"
}

// Redirected is a function that creates an AliasUsed event.
func (a Alias) Redirected() Event {
	return Event{
		Alias:      a,
		EventID:    uuid.New(),
		OccurredAt: time.Now(),
		EventType:  AliasUsed,
	}
}

// Expired is a function that creates an AliasExpired event.
func (a Alias) Expired() Event {
	return Event{
		Alias:      a,
		EventID:    uuid.New(),
		OccurredAt: time.Now(),
		EventType:  AliasExpired,
	}
}
