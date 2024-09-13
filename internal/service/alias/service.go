package alias

import (
	"context"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"sync"
)

const (
	maxGoroutines = 10
	keyLength     = 7
)

// NewAliasService creates a new alias service
func NewAliasService(aliasExpiredQ eventProducer, aliasUsedQ eventProducer, repo aliasRepo, keyGenerator keyGenerator) *AliasService {
	return &AliasService{
		aliasExpiredQ: aliasExpiredQ,
		aliasUsedQ:    aliasUsedQ,
		repo:          repo,
		keyGenerator:  keyGenerator,
	}
}

//go:generate mockery --name=aliasRepo --exported --output ./mocks --filename=alias_repo.go
type aliasRepo interface {
	SaveMany(ctx context.Context, aliases []domain.Alias) error
	FindOne(ctx context.Context, key string) (*domain.Alias, error)
	RemoveOne(ctx context.Context, key string) error
}

//go:generate mockery --name=eventProducer  --exported --output ./mocks --filename=event_producer.go
type eventProducer interface {
	Produce(event any)
}

//go:generate mockery --name=keyGenerator  --exported --output ./mocks --filename=key_generator.go
type keyGenerator interface {
	Generate(n int) (string, error)
}

type AliasService struct {
	repo          aliasRepo
	aliasExpiredQ eventProducer
	aliasUsedQ    eventProducer
	keyGenerator  keyGenerator
}

func (s *AliasService) Name() string {
	return "AliasService"
}

// CreateMany creates a set of shortened links for the given origin links
func (s *AliasService) CreateMany(ctx context.Context, requests []domain.AliasCreationRequest) ([]domain.Alias, error) {
	const fn = "AliasService::CreateMany"
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

	for errVal := range errChan {
		if errVal != nil {
			return nil, errVal
		}
	}
	aliases := make([]domain.Alias, len(requests))
	for entry := range resultChan {
		aliases[entry.index] = entry.alias
	}

	if err := s.repo.SaveMany(ctx, aliases); err != nil {
		return nil, err
	}

	return aliases, nil
}

// FindOne finds the alias link
func (s *AliasService) FindOne(ctx context.Context, key string) (*domain.Alias, error) {
	const fn = "AliasService::FindOne"

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", key))
	alias, err := s.repo.FindOne(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	if alias.Params.IsPermanent {
		return alias, nil
	}

	// check if alias is expired and send event with publisher
	if alias.Params.TriesLeft == 0 {
		event := alias.Expired()

		s.aliasExpiredQ.Produce(event)

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

	s.aliasUsedQ.Produce(event)

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("publish", event.String()),
	)

	return alias, nil
}

// RemoveOne removes the alias link
func (s *AliasService) RemoveOne(ctx context.Context, key string) error {
	const fn = "AliasService::RemoveOne"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	if err := s.repo.RemoveOne(ctx, key); err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	return nil
}
