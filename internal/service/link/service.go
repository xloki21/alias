package link

import (
	"context"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/pkg/randomizer"
	"go.uber.org/zap"
	"net/url"
	"sync"
)

const (
	maxGoroutines      = 10
	randomSuffixLength = 7
)

type aliasRepo interface {
	SaveMany(ctx context.Context, aliases []*domain.Alias) error
	FindOne(ctx context.Context, alias *domain.Alias) error
	RemoveOne(ctx context.Context, alias *domain.Alias) error
}

type eventProducer interface {
	Produce(event any)
}

type AliasService struct {
	baseURLPrefix string
	repo          aliasRepo
	aliasExpiredQ eventProducer
	aliasUsedQ    eventProducer
}

func (s *AliasService) Name() string {
	return "AliasService"
}

// CreateMany creates a set of shortened links for the given origin links
func (s *AliasService) CreateMany(ctx context.Context, aliases []*domain.Alias) error {
	const fn = "AliasService::CreateMany"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.Int("aliases count", len(aliases)))

	// validate request
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxGoroutines)
	errChan := make(chan error, len(aliases))

	for index := range aliases {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			suffix, err := randomizer.GenerateRandomStringURLSafe(randomSuffixLength)
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", fn, err)
			}
			validURL, err := url.Parse(fmt.Sprintf("%s/%s", s.baseURLPrefix, suffix))
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", fn, err)
				return
			}

			aliases[index].URL = validURL
			aliases[index].IsActive = true
		}(index)
		<-semaphore
	}
	wg.Wait()

	close(semaphore)
	close(errChan)

	for errVal := range errChan {
		if errVal != nil {
			return errVal
		}
	}

	if err := s.repo.SaveMany(ctx, aliases); err != nil {
		return err
	}

	return nil
}

// FindOne finds the alias link
func (s *AliasService) FindOne(ctx context.Context, linkID string) (*domain.Alias, error) {
	const fn = "AliasService::FindOne"

	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("id", linkID))

	urlString := fmt.Sprintf("%s/%s", s.baseURLPrefix, linkID)
	validURL, _ := url.Parse(urlString)
	alias := &domain.Alias{URL: validURL}

	if err := s.repo.FindOne(ctx, alias); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	if alias.IsPermanent {
		return alias, nil
	}

	// check if alias is expired and send event with publisher
	if alias.TTL == 0 {
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
		zap.String("redirect", alias.Origin.String()))

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
func (s *AliasService) RemoveOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "AliasService::RemoveOne"
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))

	if err := s.repo.RemoveOne(ctx, alias); err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	return nil
}

// NewAliasService creates a new alias service
func NewAliasService(aliasExpiredQ eventProducer, aliasUsedQ eventProducer, repo aliasRepo, urlPrefix string) *AliasService {
	return &AliasService{
		baseURLPrefix: urlPrefix,
		aliasExpiredQ: aliasExpiredQ,
		aliasUsedQ:    aliasUsedQ,
		repo:          repo,
	}
}
