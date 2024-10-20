//go:generate mockery
package alias

import (
	"context"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/pkg/kafker"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	keyLength     = 8
	maxGoroutines = 10
)

type eventProducer interface {
	WriteMessage(ctx context.Context, message kafker.Message) error
	Close()
}

type Alias struct {
	repo         aliasRepo
	expiredQ     eventProducer
	usedQ        eventProducer
	keyGenerator keyGenerator
}

// NewAlias creates a new alias service
func NewAlias(expiredQ eventProducer, usedQ eventProducer, repo aliasRepo, keyGenerator keyGenerator) *Alias {

	return &Alias{
		expiredQ:     expiredQ,
		usedQ:        usedQ,
		repo:         repo,
		keyGenerator: keyGenerator,
	}
}

type aliasRepo interface {
	Save(ctx context.Context, aliases []domain.Alias) error
	Find(ctx context.Context, key string) (*domain.Alias, error)
	Remove(ctx context.Context, key string) error
}

type keyGenerator interface {
	Generate(n int) (string, error)
}

func (s *Alias) Name() string {
	return "Alias"
}

// Create creates a set of shortened links for the given origin links
func (s *Alias) Create(ctx context.Context, requests []domain.CreateRequest) ([]domain.Alias, error) {
	fn := "Create"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.Int("requests count", len(requests)))

	g := errgroup.Group{}
	g.SetLimit(maxGoroutines)

	aliases := make([]domain.Alias, len(requests))
	for index := range requests {
		index := index
		g.Go(func() error {
			key, err := s.keyGenerator.Generate(keyLength)
			if err != nil {
				return fmt.Errorf("%s: %w", fn, err)
			}
			aliases[index] = domain.Alias{
				Key:      key,
				IsActive: true,
				URL:      requests[index].URL,
				Params:   requests[index].Params,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		zap.S().WithOptions(zap.AddStacktrace(zap.PanicLevel)).
			Errorw("service",
				zap.String("name", s.Name()),
				zap.String("fn", fn),
				zap.Error(err),
			)
		return nil, err
	}

	if err := s.repo.Save(ctx, aliases); err != nil {

		zap.S().WithOptions(zap.AddStacktrace(zap.PanicLevel)).
			Errorw("service",
				zap.String("name", s.Name()),
				zap.String("fn", fn),
				zap.Error(err),
				zap.Any("aliases", aliases),
			)
		return nil, err
	}

	return aliases, nil
}

func (s *Alias) FindAlias(ctx context.Context, key string) (*domain.Alias, error) {
	fn := "FindAlias"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", key))
	alias, err := s.repo.Find(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	return alias, nil
}

func (s *Alias) Use(ctx context.Context, alias *domain.Alias) error {
	fn := "Use"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", alias.Key))

	if alias.Params.IsPermanent {
		return nil
	}

	// check if alias is expired and send event with publisher
	if alias.Params.TriesLeft == 0 {
		event := alias.Expired()
		// make proto

		msg, err := kafker.MessageFromProto(event.AsProto())
		if err != nil {
			return err // todo: fix
		}

		if err := s.expiredQ.WriteMessage(ctx, *msg); err != nil {
			return err // todo: fix
		}

		zap.S().Infow("service",
			zap.String("name", s.Name()),
			zap.String("fn", fn),
			zap.String("published", event.String()),
		)
		return domain.ErrAliasExpired
	}

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("alias", alias.Type()),
		zap.String("key", alias.Key),
		zap.Uint64("tries left", alias.Params.TriesLeft))

	// publish event
	event := alias.Redirected()
	msg, err := kafker.MessageFromProto(event.AsProto())
	if err != nil {
		return err // todo: fix
	}

	if err := s.usedQ.WriteMessage(ctx, *msg); err != nil {
		return err // todo: fix
	}

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("publish", event.String()),
	)
	return nil
}

// Remove removes the alias link
func (s *Alias) Remove(ctx context.Context, key string) error {
	fn := "Remove"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	if err := s.repo.Remove(ctx, key); err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	return nil
}
