//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/xloki21/alias/internal/app/alias"
	"github.com/xloki21/alias/internal/config"
	aliasapp2 "github.com/xloki21/alias/internal/config/aliascfg"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/tests"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"
)

var testCfg = config.AppConfig{
	Service: config.Service{
		HTTP:        "localhost:8080",
		GRPC:        "localhost:8081",
		GRPCGateway: "localhost:8082",
		BaseURL:     "http://localhost:8080",
	},
	Storage: aliasapp2.StorageConfig{
		Type: repository.MongoDB,
		MongoDB: &aliasapp2.MongoDBStorageConfig{
			Database: "aliases",
		},
	},
	LoggerConfig: aliasapp2.LoggerConfig{
		Level:    "info",
		Encoding: "console",
	},
}

const (
	testApiV1         = "/api/v1"
	testEndpointAlias = testApiV1 + "/alias"
)

func TestApi_e2e(t *testing.T) {
	ctx := context.Background()

	container, _ := tests.SetupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	connstr, err := container.ConnectionString(ctx)
	assert.NoError(t, err)

	testCfg.Storage.MongoDB.URI = connstr

	application, err := alias.New(testCfg)
	assert.NoError(t, err)

	go application.Run(ctx)

	// waiting for startup as app run in non-blocking way
	time.Sleep(3 * time.Second)
	client := &http.Client{}

	endpointAliasTarget := fmt.Sprintf("%s%s", testCfg.Service.BaseURL, testEndpointAlias)

	t.Run("Create permanent aliases should be ok", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"urls\": [{\"url\": \"http://www.ya.ru\"}]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

	})

	t.Run("Create aliases with restricted usages should be ok", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"urls\": [{\"url\": \"http://www.ya.ru\",\"maxUsageCount\": 3}]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

	})

	t.Run("Create aliases should fail: request body is invalid", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"invalid-field\": [\"http://www.ya.ru\"]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	})

	t.Run("Create aliases should fail: request body is empty", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	})

	t.Run("Remove alias should fail: key not found", func(t *testing.T) {

		key := "some-random-key"

		request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", endpointAliasTarget, key), nil)
		request.RequestURI = ""
		resp, err := client.Do(request)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Remove alias should fail: key is empty", func(t *testing.T) {

		key := ""

		request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", endpointAliasTarget, key), nil)
		request.RequestURI = ""
		resp, err := client.Do(request)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Remove alias should be ok", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"urls\": [{\"url\": \"http://www.ya.ru\",\"maxUsageCount\": 3}]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		content, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		type AliasesList struct {
			Urls []string `json:"urls"`
		}

		aliases := new(AliasesList)

		err = json.Unmarshal(content, aliases)
		assert.NoError(t, err)

		key := path.Base(aliases.Urls[0])

		request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", endpointAliasTarget, key), nil)
		request.RequestURI = ""
		resp, err = client.Do(request)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("Redirect should be ok", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"urls\": [{\"url\": \"http://www.ya.ru\",\"maxUsageCount\": 3}]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		content, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		type AliasesList struct {
			Urls []string `json:"urls"`
		}

		aliases := new(AliasesList)

		err = json.Unmarshal(content, aliases)
		assert.NoError(t, err)

		resp, err = client.Get(aliases.Urls[0])
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Redirect should fail after alias has been expired", func(t *testing.T) {
		maxUsageCount := 3
		contentStr := fmt.Sprintf("{\"urls\": [{\"url\": \"http://www.ya.ru\",\"maxUsageCount\": %d}]}", maxUsageCount)
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader(contentStr))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		content, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		type AliasesList struct {
			Urls []string `json:"urls"`
		}

		aliases := new(AliasesList)

		err = json.Unmarshal(content, aliases)
		assert.NoError(t, err)

		for i := 0; i < maxUsageCount; i++ {
			resp, err = client.Get(aliases.Urls[0])
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		resp, err = client.Get(aliases.Urls[0])
		assert.NoError(t, err)
		assert.Equal(t, http.StatusGone, resp.StatusCode)

	})

}
