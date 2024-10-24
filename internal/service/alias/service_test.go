package alias

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/service/alias/mocks"
	"net/url"
	"testing"
)

type TestHelper struct {
	expiredQ *mocks.MockEventProducer
	usedQ    *mocks.MockEventProducer
	repo     *mocks.MockAliasRepo
	keyGen   *mocks.MockKeyGenerator
	service  *Alias
}

func NewTestHelper(t *testing.T) *TestHelper {
	repo := mocks.NewMockAliasRepo(t)
	expiredQ := mocks.NewMockEventProducer(t)
	usedQ := mocks.NewMockEventProducer(t)
	keyGen := mocks.NewMockKeyGenerator(t)
	return &TestHelper{
		expiredQ: expiredQ,
		usedQ:    usedQ,
		repo:     repo,
		keyGen:   keyGen,
		service:  NewAlias(expiredQ, usedQ, repo, keyGen)}
}

func TestAlias_Create(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx      context.Context
		requests []domain.CreateRequest
	}

	testCases := []struct {
		name      string
		args      args
		mockFunc  func(*TestHelper, args) []domain.Alias
		expectErr error
	}{
		{
			name: "create many aliases successfully",
			args: args{
				ctx:      context.Background(),
				requests: TestSetAliasCreationRequests(10),
			},
			mockFunc: func(th *TestHelper, args args) []domain.Alias {
				randomKey := "random-key"
				for index := 0; index < len(args.requests); index++ {
					th.keyGen.On("Generate", keyLength).Return(randomKey, nil)
				}

				aliases := make([]domain.Alias, len(args.requests))
				for index, request := range args.requests {
					aliases[index] = domain.Alias{
						Key:      randomKey,
						URL:      request.URL,
						IsActive: true,
						Params:   request.Params,
					}
				}

				th.repo.On("Save", args.ctx, aliases).
					Return(nil)

				return aliases
			},
			expectErr: nil,
		},
		{
			name: "create many aliases failed due to key generation failure",
			args: args{
				ctx:      context.Background(),
				requests: TestSetAliasCreationRequests(10),
			},
			mockFunc: func(th *TestHelper, args args) []domain.Alias {
				for index := 0; index < len(args.requests); index++ {
					th.keyGen.On("Generate", keyLength).Return("", assert.AnError)
				}

				return nil
			},
			expectErr: assert.AnError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			th := NewTestHelper(t)
			expectedAliases := testCase.mockFunc(th, testCase.args)

			gotResult, gotErr := th.service.Create(testCase.args.ctx, testCase.args.requests)
			require.ErrorIs(t, gotErr, testCase.expectErr)
			require.Equal(t, expectedAliases, gotResult)
		})
	}
}

func TestAlias_FindAlias(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx context.Context
		key string
	}

	tests := []struct {
		name      string
		args      args
		mockFunc  func(*TestHelper, args) *domain.Alias
		expectErr error
	}{
		{
			name: "original url found successfully",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				alias := &domain.Alias{
					ID:       "unique-id",
					Key:      args.key,
					URL:      &url.URL{Scheme: "http", Host: "www.host.test", Path: "/path"},
					IsActive: true,
					Params:   domain.TTLParams{IsPermanent: true},
				}

				th.repo.On("Find", args.ctx, args.key).Return(alias, nil)

				return alias
			},
		},
		{
			name: "original url not found",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.repo.On("Find", args.ctx, args.key).Return(nil, domain.ErrAliasNotFound)
				return nil
			},
			expectErr: domain.ErrAliasNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			th := NewTestHelper(t)
			wants := tt.mockFunc(th, tt.args)
			got, err := th.service.FindAlias(tt.args.ctx, tt.args.key)
			assert.Equal(t, wants, got)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}
}

func TestAlias_Use(t *testing.T) {
	t.Parallel()

	testData := []domain.Alias{
		TestExpiredAlias(t),
		TestAlias(t, false),
		TestAlias(t, true),
	}

	type args struct {
		ctx   context.Context
		alias *domain.Alias
	}
	tests := []struct {
		name      string
		args      args
		mockFunc  func(*TestHelper, args) *domain.Alias
		expectErr error
	}{
		{
			name: "use expired alias",
			args: args{ctx: context.Background(), alias: &testData[0]},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.expiredQ.On("WriteMessage", context.Background(), mock.AnythingOfType("AliasExpired")).Return(nil)
				return nil
			},
			expectErr: domain.ErrAliasExpired,
		},
		{
			name: "use valid alias with ttl successfully",
			args: args{ctx: context.Background(), alias: &testData[1]},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.usedQ.On("WriteMessage", context.Background(), mock.AnythingOfType("AliasUsed")).Return(nil)
				return args.alias
			},
		},
		{
			name: "use valid permanent alias",
			args: args{ctx: context.Background(), alias: &testData[2]},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				return args.alias
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			th := NewTestHelper(t)
			err := th.service.Use(tt.args.ctx, tt.args.alias)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}
}

func TestAlias_Remove(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx context.Context
		key string
	}

	tests := []struct {
		name      string
		args      args
		mockFunc  func(*TestHelper, args) *domain.Alias
		expectErr error
	}{
		{
			name: "remove alias successfully",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.repo.On("Remove", args.ctx, args.key).Return(nil)
				return nil
			},
		},
		{
			name: "alias not found on remove",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.repo.On("Remove", args.ctx, args.key).Return(domain.ErrAliasNotFound)
				return nil
			},
			expectErr: domain.ErrAliasNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			th := NewTestHelper(t)
			tt.mockFunc(th, tt.args)
			err := th.service.Remove(tt.args.ctx, tt.args.key)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}

}
