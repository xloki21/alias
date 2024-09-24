//go:generate mockery
package aliassvc

import (
	"context"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"sync"
)

const (
	keyLength     = 8
	maxGoroutines = 10
)

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

type eventProducer interface {
	Produce(event any)
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

	type indexedResult struct {
		index int
		alias domain.Alias
	}

	// validate request
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxGoroutines)
	errChan := make(chan error, len(requests))
	resultChan := make(chan indexedResult, len(requests))
	for index := range requests {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			key, err := s.keyGenerator.Generate(keyLength)
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", fn, err)
			}

			resultChan <- indexedResult{
				index: index,
				alias: domain.Alias{
					Key:      key,
					IsActive: true,
					URL:      requests[index].URL,
					Params:   requests[index].Params,
				},
			}

		}(index)
		<-semaphore
	}
	wg.Wait()

	close(semaphore)
	close(errChan)
	close(resultChan)

	for err := range errChan {
		if err != nil {
			zap.S().WithOptions(zap.AddStacktrace(zap.PanicLevel)).
				Errorw("service",
					zap.String("name", s.Name()),
					zap.String("fn", fn),
					zap.Error(err),
				)
			return nil, err
		}
	}
	aliases := make([]domain.Alias, len(requests))
	for entry := range resultChan {
		aliases[entry.index] = entry.alias
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

func (s *Alias) FindOriginalURL(ctx context.Context, key string) (*domain.Alias, error) {
	fn := "FindOriginalURL"
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

func (s *Alias) Use(ctx context.Context, alias *domain.Alias) (*domain.Alias, error) {
	fn := "Use"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", alias.Key))

	if alias.Params.IsPermanent {
		return alias, nil
	}

	// check if alias is expired and send event with publisher
	if alias.Params.TriesLeft == 0 {
		event := alias.Expired()

		s.expiredQ.Produce(event)

		zap.S().Infow("service",
			zap.String("name", s.Name()),
			zap.String("fn", fn),
			zap.String("published", event.String()),
		)
		return nil, domain.ErrAliasExpired
	}

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("alias", alias.Type()),
		zap.String("key", alias.Key),
		zap.Int("tries left", alias.Params.TriesLeft))

	// publish event
	event := alias.Redirected()

	s.usedQ.Produce(event)

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("publish", event.String()),
	)

	return alias, nil
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
