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
	"github.com/xloki21/alias/internal/app"
	"github.com/xloki21/alias/internal/controller/rest"
	"github.com/xloki21/alias/internal/controller/rest/mw"
	"github.com/xloki21/alias/internal/tests"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"
)

const (
	testAppHttpServiceAddress = "localhost:8080"
	baseUrlPrefix             = "http://" + testAppHttpServiceAddress
	testApiV1                 = "/api/v1"
	testEndpointAlias         = testApiV1 + "/alias"
	testEndpointHealthcheck   = testApiV1 + "/healthcheck"
	testEndpointRedirect      = ""
)

func TestApi_e2e(t *testing.T) {
	ctx := context.Background()

	container, db := tests.SetupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	service := tests.NewTestAliasService(ctx, db)
	ctrl := rest.NewController(service, baseUrlPrefix)

	mux := http.NewServeMux()
	mux.HandleFunc(testEndpointAlias, mw.Use(ctrl.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointHealthcheck, mw.Use(ctrl.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointAlias+"/{key}", mw.Use(ctrl.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointRedirect+"/{key}", mw.Use(ctrl.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))

	application := &app.Application{
		HttpServer: &http.Server{
			Addr:    testAppHttpServiceAddress,
			Handler: mux,
		},
	}

	go func() {
		if err := application.Run(ctx); err != nil {
			zap.S().Fatal("failed to start application", zap.Error(err))
		}
	}()

	client := &http.Client{Timeout: 10 * time.Second}

	endpointAliasTarget := fmt.Sprintf("http://%s%s", testAppHttpServiceAddress, testEndpointAlias)

	t.Run("Create aliases should be ok", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"urls\": [\"http://www.ya.ru\"]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

	})
	t.Run("Create aliases should fail: request body is invalid", func(t *testing.T) {
		resp, err := client.Post(endpointAliasTarget,
			"application/json", strings.NewReader("{\"fail-key\": [\"http://www.ya.ru\"]}"))
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
			"application/json", strings.NewReader("{\"urls\": [\"http://www.ya.ru\"]}"))
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
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", testAppHttpServiceAddress, testEndpointAlias),
			"application/json", strings.NewReader("{\"urls\": [\"http://www.ya.ru\"]}"))
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
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s?maxUsageCount=%d", testAppHttpServiceAddress, testEndpointAlias, maxUsageCount),
			"application/json", strings.NewReader("{\"urls\": [\"http://www.ya.ru\"]}"))
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
