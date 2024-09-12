package domain

import (
	"fmt"
	"time"
)

// AliasUsed is a struct that represents an alias link redirect event.
type AliasUsed struct {
	Alias
	OccurredAt time.Time
}

func (a AliasUsed) String() string {
	return fmt.Sprintf("%s: %s", "AliasUsed", a.Key)
}

// AliasExpired is a struct that represents an alias link expired event.
type AliasExpired struct {
	Alias
	OccurredAt time.Time
}

func (u AliasExpired) String() string {
	return fmt.Sprintf("%s: %s", "AliasExpired", u.Key)
}
