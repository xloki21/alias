//go:build integration
// +build integration

package integration

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/services/aliassvc"
	"github.com/xloki21/alias/internal/tests"
	"testing"
)

func TestAlias_Create_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := tests.SetupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewTestAliasService(ctx, db)

	type args struct {
		ctx      context.Context
		requests []domain.CreateRequest
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "create multiple aliases with success",
			args:        args{ctx: context.Background(), requests: aliassvc.TestSetAliasCreationRequests(2000)},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := aliasService.Create(testCase.args.ctx, testCase.args.requests)
			assert.NoError(t, err)
		})
	}
}

func TestAlias_FindByKey_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		aliassvc.TestAlias(t, false),
		aliassvc.TestAlias(t, true),
	}

	container, db := tests.SetupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewTestAliasService(ctx, db)

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
			got, err := aliasService.FindByKey(ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
			if testCase.wants != nil {
				assert.Equal(t, testCase.wants.URL, got.URL)
			}
		})
	}
}

func TestAlias_Remove_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		aliassvc.TestAlias(t, false),
		aliassvc.TestAlias(t, true),
	}

	container, db := tests.SetupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewTestAliasService(ctx, db)

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
			err := aliasService.Remove(testCase.args.ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
		})
	}
}
