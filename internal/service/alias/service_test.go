//go:build mock
// +build mock

package alias

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xloki21/alias/internal/domain"
	"net/url"
	"testing"
)

func TestService_CreateMany(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx      context.Context
		requests []domain.AliasCreationRequest
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

				th.repo.On("SaveMany", args.ctx, aliases).
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

			gotResult, gotErr := th.service.CreateMany(testCase.args.ctx, testCase.args.requests)
			require.ErrorIs(t, gotErr, testCase.expectErr)
			require.Equal(t, expectedAliases, gotResult)
		})
	}
}

func TestService_FindOne(t *testing.T) {
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
			name: "find alias successfully",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				alias := &domain.Alias{
					ID:       "unique-id",
					Key:      args.key,
					URL:      &url.URL{Scheme: "http", Host: "www.host.test", Path: "/path"},
					IsActive: true,
					Params:   domain.TTLParams{IsPermanent: true},
				}

				th.repo.On("FindOne", args.ctx, args.key).Return(alias, nil)

				return alias
			},
		},
		{
			name: "find ttl-restricted alias successfully",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				alias := &domain.Alias{
					ID:       "unique-id",
					Key:      args.key,
					URL:      &url.URL{Scheme: "http", Host: "www.host.test", Path: "/path"},
					IsActive: true,
					Params:   domain.TTLParams{TriesLeft: 3, IsPermanent: false},
				}

				th.repo.On("FindOne", args.ctx, args.key).Return(alias, nil)
				th.aliasUsedQ.On("Produce", mock.AnythingOfType("AliasUsed"))
				return alias
			},
		},
		{
			name: "find expired alias",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				alias := &domain.Alias{
					ID:       "unique-id",
					Key:      args.key,
					URL:      &url.URL{Scheme: "http", Host: "www.host.test", Path: "/path"},
					IsActive: true,
					Params:   domain.TTLParams{TriesLeft: 0},
				}

				th.repo.On("FindOne", args.ctx, args.key).Return(alias, nil)
				th.aliasExpiredQ.On("Produce", mock.AnythingOfType("AliasExpired"))
				return nil
			},
			expectErr: domain.ErrAliasExpired,
		},
		{
			name: "alias not found",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.repo.On("FindOne", args.ctx, args.key).Return(nil, domain.ErrAliasNotFound)
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
			got, err := th.service.FindOne(tt.args.ctx, tt.args.key)
			assert.Equal(t, wants, got)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}
}

func TestService_RemoveOne(t *testing.T) {
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
				th.repo.On("RemoveOne", args.ctx, args.key).Return(nil)
				return nil
			},
		},
		{
			name: "alias not found on remove",
			args: args{ctx: context.Background(), key: "lookup-key"},
			mockFunc: func(th *TestHelper, args args) *domain.Alias {
				th.repo.On("RemoveOne", args.ctx, args.key).Return(domain.ErrAliasNotFound)
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
			err := th.service.RemoveOne(tt.args.ctx, tt.args.key)
			assert.ErrorIs(t, err, tt.expectErr)
		})
	}

}
