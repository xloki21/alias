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
	"github.com/xloki21/alias/tests"
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
			args:        args{ctx: context.Background(), requests: alias.TestSetAliasCreationRequests(2000)},
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

func TestAlias_FindAlias_MongoDB(t *testing.T) {
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

	aliasService := tests.NewTestAliasService(ctx, db)

	type args struct {
		ctx context.Context
		key string
	}

	tests := []struct {
		name      string
		args      args
		wants     *domain.Alias
		expectErr error
	}{
		{
			name:      "alias found successfully",
			args:      args{ctx: context.Background(), key: testData[0].Key},
			wants:     &testData[0],
			expectErr: nil,
		},
		{
			name:      "alias not found",
			args:      args{ctx: context.Background(), key: "lookup-key"},
			wants:     nil,
			expectErr: domain.ErrAliasNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := aliasService.FindAlias(ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectErr)
			if testCase.wants != nil {
				assert.Equal(t, testCase.wants.URL, got.URL)
			}
		})
	}
}

func TestAlias_Use_MongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		alias.TestAlias(t, false),
		alias.TestAlias(t, true),
		alias.TestExpiredAlias(t),
	}
	testData[2].Params.TriesLeft = 0

	container, db := tests.SetupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := tests.NewTestAliasService(ctx, db)

	type args struct {
		ctx   context.Context
		alias *domain.Alias
	}
	tests := []struct {
		name      string
		args      args
		wants     *domain.Alias
		expectErr error
	}{
		{
			name:      "use expired alias",
			args:      args{ctx: context.Background(), alias: &testData[2]},
			wants:     nil,
			expectErr: domain.ErrAliasExpired,
		},
		{
			name:  "use valid alias with ttl successfully",
			args:  args{ctx: context.Background(), alias: &testData[0]},
			wants: &testData[0],
		},
		{
			name:  "use valid permanent alias",
			args:  args{ctx: context.Background(), alias: &testData[1]},
			wants: &testData[1],
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := aliasService.Use(testCase.args.ctx, testCase.args.alias)
			assert.ErrorIs(t, err, testCase.expectErr)
		})
	}
}

func TestAlias_Remove_MongoDB(t *testing.T) {
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
