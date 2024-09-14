package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/xloki21/alias/internal/app"
	"github.com/xloki21/alias/internal/controller"
	"github.com/xloki21/alias/internal/controller/mw"
	"github.com/xloki21/alias/internal/tests"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	testAppAddress          = "localhost:8080"
	testApiV1               = "/api/v1"
	testEndpointCreateAlias = testApiV1 + "/alias"
	testEndpointHealthcheck = testApiV1 + "/healthcheck"
	testEndpointRedirect    = ""
	testEndpointRemoveLink  = testApiV1 + "/remove"
)

func TestApi_e2e(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	container, db := tests.SetupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	service := tests.NewAliasTestService(ctx, db)
	ctrl := controller.NewAliasController(service, fmt.Sprintf("http://%s", testAppAddress))

	mux := http.NewServeMux()
	mux.HandleFunc(testEndpointCreateAlias, mw.Use(ctrl.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointHealthcheck, mw.Use(ctrl.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointRemoveLink, mw.Use(ctrl.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(testEndpointRedirect+"/{key}", mw.Use(ctrl.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))

	application := &app.Application{
		Address:    testAppAddress,
		Router:     mux,
		Controller: ctrl,
	}

	go func() {
		if err := application.Run(context.Background()); err != nil {
			zap.S().Fatal("failed to start application", zap.Error(err))
		}
	}()

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("CreateAlias should return alias successfully", func(t *testing.T) {
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", application.Address, testEndpointCreateAlias),
			"application/json", strings.NewReader("{\"urls\": [\"http://www.ya.ru\"]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

	})
	t.Run("CreateAlias should fail: request body is invalid", func(t *testing.T) {
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", application.Address, testEndpointCreateAlias),
			"application/json", strings.NewReader("{\"urlcs\": [\"http://www.ya.ru\"]}"))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	})

	t.Run("CreateAlias should fail: request body is empty", func(t *testing.T) {
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", application.Address, testEndpointCreateAlias),
			"application/json", nil)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	})

	t.Run("Redirect should be ok", func(t *testing.T) {
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", application.Address, testEndpointCreateAlias),
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

}
