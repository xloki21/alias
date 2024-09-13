package alias

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xloki21/alias/internal/domain"
	"testing"
)

func TestAliasService_CreateMany(t *testing.T) {

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
