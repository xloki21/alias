//go:build integration
// +build integration

package integration

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/service/alias"
	"github.com/xloki21/alias/internal/tests"
	"testing"
)

func TestAliasService_CreateMany_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := tests.SetupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewAliasTestService(ctx, db)

	type args struct {
		ctx      context.Context
		requests []domain.AliasCreationRequest
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "create multiple aliases with success",
			args:        args{ctx: context.Background(), requests: alias.TestSetAliasCreationRequests(2000)},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := aliasService.CreateMany(testCase.args.ctx, testCase.args.requests)
			assert.NoError(t, err)
		})
	}
}

func TestAliasService_FindOne_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		alias.TestAlias(t, false),
		alias.TestAlias(t, true),
	}

	container, db := tests.SetupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewAliasTestService(ctx, db)

	type args struct {
		ctx context.Context
		key string
	}

	testCases := []struct {
		name        string
		args        args
		wants       *domain.Alias
		expectedErr error
	}{
		{
			name: "alias is not found",
			args: args{
				ctx: context.Background(),
				key: "non-existent-key",
			},
			wants:       nil,
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name: "alias is found",
			args: args{
				ctx: context.Background(),
				key: testData[0].Key,
			},
			wants:       &testData[0],
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := aliasService.FindOne(ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
			if testCase.wants != nil {
				assert.Equal(t, testCase.wants.URL, got.URL)
			}
		})
	}
}

func TestAliasService_RemoveOne_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		alias.TestAlias(t, false),
		alias.TestAlias(t, true),
	}

	container, db := tests.SetupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewAliasTestService(ctx, db)

	type args struct {
		ctx context.Context
		key string
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "remove non-existent aliases",
			args:        args{ctx: context.Background(), key: "non-existent-key"},
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name:        "remove alias successfully",
			args:        args{ctx: context.Background(), key: testData[0].Key},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := aliasService.RemoveOne(testCase.args.ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
		})
	}
}
