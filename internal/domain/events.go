package domain

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/url"
	"time"
)

const (
	AliasExpired = "alias expired"
	AliasUsed    = "alias used"
)

type Event struct {
	Alias
	EventID    uuid.UUID
	EventType  string
	OccurredAt time.Time
}

func (e Event) String() string {
	return e.EventType
}

func (e Event) AsProto() *aliasapi.Event {
	var eventType aliasapi.EventType
	switch e.EventType {
	case AliasUsed:
		eventType = aliasapi.EventType_EVENT_TYPE_USED
	case AliasExpired:
		eventType = aliasapi.EventType_EVENT_TYPE_EXPIRED
	default:
		eventType = aliasapi.EventType_EVENT_TYPE_UNSPECIFIED
	}

	return &aliasapi.Event{
		Alias: &aliasapi.Alias{
			Id:       e.Alias.ID,
			Key:      e.Alias.Key,
			Url:      e.Alias.URL.String(),
			IsActive: e.Alias.IsActive,
			Params: &aliasapi.AliasParams{
				TriesLeft:   e.Alias.Params.TriesLeft,
				IsPermanent: e.Alias.Params.IsPermanent,
			},
		},
		EventId:    e.EventID.String(),
		EventType:  eventType,
		OccurredAt: timestamppb.New(e.OccurredAt),
	}
}

func NewEventFromProto(e *aliasapi.Event) (*Event, error) {

	originURL, err := url.Parse(e.Alias.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid url format: %w", err)
	}
	domainAlias := Alias{
		ID:       e.Alias.Id,
		Key:      e.Alias.Key,
		URL:      originURL,
		IsActive: e.Alias.IsActive,
		Params:   TTLParams{e.Alias.Params.TriesLeft, e.Alias.Params.IsPermanent},
	}

	var eventType string

	switch e.EventType {
	case aliasapi.EventType_EVENT_TYPE_USED:
		eventType = AliasUsed
	case aliasapi.EventType_EVENT_TYPE_EXPIRED:
		eventType = AliasExpired
	default:
		panic("unknown event type")
	}

	return &Event{
		Alias:      domainAlias,
		EventID:    uuid.MustParse(e.EventId),
		EventType:  eventType,
		OccurredAt: e.GetOccurredAt().AsTime(),
	}, nil
}
