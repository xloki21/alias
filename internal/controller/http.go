package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

const maxGoroutines = 10

type aliasService interface {
	CreateMany(ctx context.Context, aliases []*domain.Alias) error
	FindOne(ctx context.Context, linkID string) (*domain.Alias, error)
	RemoveOne(ctx context.Context, alias *domain.Alias) error
}

type AliasController struct {
	address string
	service aliasService
}

func NewAliasController(service aliasService, address string) *AliasController {
	return &AliasController{service: service, address: address}
}

func (ac *AliasController) CreateAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// parse query params
	query := r.URL.Query()
	var isPermanent bool
	var TTLValue int
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
		TTLValue = int(value)
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

	// helper struct to keep order of the validated URL's
	type indexedResult struct {
		index int
		alias *domain.Alias
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
			resultChan <- indexedResult{index: index, alias: &domain.Alias{
				Origin:      validURL,
				TTL:         TTLValue,
				IsActive:    true,
				IsPermanent: isPermanent,
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
	aliases := make([]*domain.Alias, len(payload.URLs), len(payload.URLs))
	for item := range resultChan {
		aliases[item.index] = item.alias
	}

	if err := ac.service.CreateMany(r.Context(), aliases); err != nil {
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

func (ac *AliasController) Redirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	linkID := r.PathValue("linkID")
	alias, err := ac.service.FindOne(r.Context(), linkID)
	if err != nil {
		if errors.Is(err, domain.ErrAliasNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else if errors.Is(err, domain.ErrAliasExpired) {
			http.Error(w, "url expired", http.StatusGone)
		} else {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}
		return
	}
	http.Redirect(w, r, alias.Origin.String(), http.StatusTemporaryRedirect)
}

func (ac *AliasController) RemoveAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	payload := new(requestDeleteAlias)
	if err := json.Unmarshal(content, payload); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	alias := domain.Alias{Key: payload.Key}
	if err := ac.service.RemoveOne(r.Context(), &alias); err != nil {
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
func (ac *AliasController) Healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
