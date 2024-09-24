package httpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

const maxGoroutines = 10

type aliasService interface {
	Create(ctx context.Context, requests []domain.CreateRequest) ([]domain.Alias, error)
	FindOriginalURL(ctx context.Context, key string) (*domain.Alias, error)
	Use(ctx context.Context, alias *domain.Alias) (*domain.Alias, error)
	Remove(ctx context.Context, key string) error
}

type requestURLList struct {
	URLs []string `json:"urls"`
}

type responseURLList struct {
	URLs []string `json:"urls"`
}

// helper struct to keep order of the validated URL's
type indexedResult struct {
	index   int
	request domain.CreateRequest
}

type Controller struct {
	address string
	service aliasService
}

func NewController(service aliasService, address string) *Controller {
	return &Controller{service: service, address: address}
}

func (ac *Controller) CreateAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// parse query params
	query := r.URL.Query()
	var isPermanent bool
	var triesLeftValue int
	if maxUsageCount, ok := query["maxUsageCount"]; !ok {
		isPermanent = true
	} else {
		if len(maxUsageCount) != 1 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		value, err := strconv.ParseInt(maxUsageCount[0], 10, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		triesLeftValue = int(value)
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	payload := &requestURLList{}
	if err := json.Unmarshal(content, payload); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(payload.URLs) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// validate request
	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, maxGoroutines)
	errChan := make(chan error, len(payload.URLs))
	resultChan := make(chan indexedResult, len(payload.URLs))
	for index, urlString := range payload.URLs {
		semaphore <- struct{}{}
		wg.Add(1)
		go func(index int, urlString string) {
			defer wg.Done()
			validURL, err := url.Parse(urlString)
			if err != nil {
				errChan <- err
			}
			resultChan <- indexedResult{index: index, request: domain.CreateRequest{
				Params: domain.TTLParams{TriesLeft: triesLeftValue, IsPermanent: isPermanent},
				URL:    validURL,
			}}

		}(index, urlString)
		<-semaphore
	}
	wg.Wait()

	close(semaphore)
	close(resultChan)
	close(errChan)

	for errVal := range errChan {
		if errVal != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}
	requests := make([]domain.CreateRequest, len(payload.URLs), len(payload.URLs))
	for entry := range resultChan {
		requests[entry.index] = entry.request
	}

	aliases, err := ac.service.Create(r.Context(), requests)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	response := responseURLList{URLs: make([]string, len(payload.URLs))}
	for index := range aliases {
		response.URLs[index] = fmt.Sprintf("%s/%s", ac.address, aliases[index].Key)
	}

	answer, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(answer); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	return
}

func (ac *Controller) Redirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	key := r.PathValue("key")

	alias, err := ac.service.FindOriginalURL(r.Context(), key)

	if err != nil {
		if errors.Is(err, domain.ErrAliasNotFound) {
			zap.S().Error("alias not found", zap.String("key", key))
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	alias, err = ac.service.Use(r.Context(), alias)

	if err != nil {
		if errors.Is(err, domain.ErrAliasExpired) {
			http.Error(w, "url expired", http.StatusGone)
		} else {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}
		return
	}
	http.Redirect(w, r, alias.URL.String(), http.StatusTemporaryRedirect)
}

func (ac *Controller) RemoveAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	key := r.PathValue("key")

	if err := ac.service.Remove(r.Context(), key); err != nil {
		if errors.Is(err, domain.ErrAliasNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Healthcheck endpoint
func (ac *Controller) Healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
