package domain

import (
	"time"
)

const (
	aliasExpiredEventType = "alias expired"
	aliasUsedEventType    = "alias used"
)

type Event struct {
	Alias
	eventType  string
	OccurredAt time.Time
}

func (e Event) String() string {
	return e.eventType
}
