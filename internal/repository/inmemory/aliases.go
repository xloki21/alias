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

// SaveOne saves a alias link
func (a *AliasRepository) SaveOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "in-memory::SaveOne"
	a.mu.Lock()
	defer a.mu.Unlock()
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()),
		zap.String("origin", alias.Origin.String()))
	id := alias.URL.String()
	alias.ID = id
	a.db[id] = alias
	return nil
}

// SaveMany saves many aliases in one run
func (a *AliasRepository) SaveMany(ctx context.Context, aliases []*domain.Alias) error {
	const fn = "in-memory::SaveMany"
	a.mu.Lock()
	defer a.mu.Unlock()
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.Int("alias count", len(aliases)))
	for index := range aliases {
		id := aliases[index].URL.String()
		aliases[index].ID = id

		a.db[id] = aliases[index]
	}
	return nil
}

// FindOne gets the target link from the shortened one
func (a *AliasRepository) FindOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "in-memory::FindOne"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))

	a.mu.RLock()
	defer a.mu.RUnlock()
	if presented, ok := a.db[alias.URL.String()]; ok {
		*alias = *presented
		return nil
	} else {
		return domain.ErrAliasNotFound
	}
}

// RemoveOne removes a shortened link
func (a *AliasRepository) RemoveOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "in-memory::RemoveOne"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.db[alias.URL.String()]; ok {
		delete(a.db, alias.URL.String())
		return nil
	} else {
		return domain.ErrAliasNotFound
	}
}

func (a *AliasRepository) DecreaseTTLCounter(ctx context.Context, alias domain.Alias) error {
	const fn = "in-memory::DecreaseTTLCounter"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))
	a.mu.Lock()
	defer a.mu.Unlock()

	if alias.TTL == 0 {
		return domain.ErrAliasExpired
	}

	if _, ok := a.db[alias.URL.String()]; !ok { // check: possibly unnecessary
		return domain.ErrAliasNotFound
	}

	// decrease TTL counter
	a.db[alias.URL.String()].TTL -= 1
	return nil
}

func NewAliasRepository() *AliasRepository {
	return &AliasRepository{
		db: make(map[string]*domain.Alias),
		mu: sync.RWMutex{},
	}
}
