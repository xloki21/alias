package aliassvc

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/pkg/keygen"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
	"net/url"
	"testing"
)

func TestSetAliasCreationRequests(quantity int) []domain.CreateRequest {
	requests := make([]domain.CreateRequest, quantity)

	for i := 0; i < quantity; i++ {
		triesLeft := uint64(rand.Intn(10))
		isPermanent := true
		if triesLeft == 0 {
			isPermanent = false
		}
		requests[i] = domain.CreateRequest{
			URL: &url.URL{
				Scheme: "http",
				Host:   fmt.Sprintf("host%d.test", i),
			},
			Params: domain.TTLParams{TriesLeft: triesLeft, IsPermanent: isPermanent},
		}
	}
	return requests
}

func TestAlias(t *testing.T, isPermanent bool) domain.Alias {
	triesLeft := uint64(1 + rand.Intn(10))
	if isPermanent {
		triesLeft = 0
	}

	key, err := keygen.NewURLSafeRandomStringGenerator().Generate(keyLength)
	assert.NoError(t, err)

	return domain.Alias{
		ID:       primitive.NewObjectID().Hex(),
		Key:      key,
		URL:      &url.URL{Scheme: "http", Host: "host.test"},
		IsActive: true,
		Params:   domain.TTLParams{TriesLeft: triesLeft, IsPermanent: isPermanent},
	}
}

func TestExpiredAlias(t *testing.T) domain.Alias {
	alias := TestAlias(t, false)
	alias.Params.TriesLeft = 0
	return alias
}
