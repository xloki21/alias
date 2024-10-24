package httpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"net/url"
)

const maxGoroutines = 10

type aliasService interface {
	Create(ctx context.Context, requests []domain.CreateRequest) ([]domain.Alias, error)
	FindAlias(ctx context.Context, key string) (*domain.Alias, error)
	Use(ctx context.Context, alias *domain.Alias) error
	Remove(ctx context.Context, key string) error
}

type requestURLList struct {
	URLs []struct {
		Url           string `json:"url"`
		MaxUsageCount uint64 `json:"maxUsageCount,omitempty"`
	} `json:"urls"`
}

type responseURLList struct {
	URLs []string `json:"urls"`
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

	g := errgroup.Group{}
	g.SetLimit(maxGoroutines)
	requests := make([]domain.CreateRequest, len(payload.URLs), len(payload.URLs))
	for index, singleURL := range payload.URLs {
		index := index
		singleURL := singleURL
		g.Go(func() error {
			requests[index] = domain.CreateRequest{}
			validURL, err := url.Parse(singleURL.Url)
			if err != nil {
				return domain.ErrInvalidURLFormat
			}

			requests[index] = domain.CreateRequest{
				Params: domain.TTLParams{TriesLeft: singleURL.MaxUsageCount, IsPermanent: singleURL.MaxUsageCount <= 0},
				URL:    validURL,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		if errors.Is(err, domain.ErrInvalidURLFormat) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
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

	alias, err := ac.service.FindAlias(r.Context(), key)

	if err != nil {
		if errors.Is(err, domain.ErrAliasNotFound) {
			//zap.S().Error("alias not found", zap.String("key", key))
			zap.S().WithOptions(zap.AddStacktrace(zap.DPanicLevel)).Errorw("HTTP", zap.String("error", domain.ErrAliasNotFound.Error()))

			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			zap.S().WithOptions(zap.AddStacktrace(zap.DPanicLevel)).Errorw("HTTP", zap.String("error", "internal server error"))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	err = ac.service.Use(r.Context(), alias)
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
