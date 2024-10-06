package domain

import (
	"time"
)

const (
	aliasExpired = "alias expired"
	aliasUsed    = "alias used"
)

type Event struct {
	Alias
	EventType  string
	OccurredAt time.Time
}

func (e Event) String() string {
	return e.EventType
}
