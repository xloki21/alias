package alias

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/service/alias/mocks"
	"github.com/xloki21/alias/pkg/keygen"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
	"net/url"
	"testing"
)

type TestHelper struct {
	repo    *mocks.AliasRepo
	keyGen  *mocks.KeyGenerator
	service *Service
}

func NewTestHelper(t *testing.T) *TestHelper {
	repo := mocks.NewAliasRepo(t)
	aliasExpiredQ := mocks.NewEventProducer(t)
	aliasUsedQ := mocks.NewEventProducer(t)
	keyGen := mocks.NewKeyGenerator(t)
	return &TestHelper{
		repo:    repo,
		keyGen:  keyGen,
		service: NewAliasService(aliasExpiredQ, aliasUsedQ, repo, keyGen)}
}

func TestSetAliasCreationRequests(quantity int) []domain.AliasCreationRequest {
	requests := make([]domain.AliasCreationRequest, quantity)

	for i := 0; i < quantity; i++ {
		triesLeft := rand.Intn(10)
		isPermanent := true
		if triesLeft == 0 {
			isPermanent = false
		}
		requests[i] = domain.AliasCreationRequest{
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
	triesLeft := rand.Intn(10)
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
