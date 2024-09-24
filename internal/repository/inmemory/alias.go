package inmemory

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"sync"
)

type AliasRepository struct {
	mu sync.RWMutex
	db map[string]*domain.Alias
}

func (a *AliasRepository) Name() string {
	return "in-memory::AliasRepository"
}

// Save saves many aliases in one run
func (a *AliasRepository) Save(ctx context.Context, aliases []domain.Alias) error {
	const fn = "Save"
	a.mu.Lock()
	defer a.mu.Unlock()
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.Int("alias count", len(aliases)))
	for _, alias := range aliases {
		a.db[alias.Key] = &alias
	}
	return nil
}

// Find gets the target link from the shortened one
func (a *AliasRepository) Find(ctx context.Context, key string) (*domain.Alias, error) {
	const fn = "Find"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	a.mu.RLock()
	defer a.mu.RUnlock()
	if presented, ok := a.db[key]; ok {
		return presented, nil
	} else {
		return nil, domain.ErrAliasNotFound
	}
}

// Remove removes a shortened link
func (a *AliasRepository) Remove(ctx context.Context, key string) error {
	const fn = "Remove"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("key", key))
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.db[key]; ok {
		delete(a.db, key)
	} else {
		return domain.ErrAliasNotFound
	}
	return nil
}

func (a *AliasRepository) DecreaseTTLCounter(ctx context.Context, key string) error {
	const fn = "DecreaseTTLCounter"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("id", key))
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.db[key]; !ok {
		return domain.ErrAliasNotFound
	}
	if a.db[key].Params.TriesLeft == 0 {
		return domain.ErrAliasExpired
	}
	// decrease TTL counter
	a.db[key].Params.TriesLeft -= 1
	return nil
}

func NewAliasRepository() *AliasRepository {
	return &AliasRepository{
		db: make(map[string]*domain.Alias),
		mu: sync.RWMutex{},
	}
}
