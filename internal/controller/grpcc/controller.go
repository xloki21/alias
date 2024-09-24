package grpcc

import (
	"context"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	aliasapi "github.com/xloki21/alias/internal/gen/go/pbuf/alias"
	"github.com/xloki21/alias/pkg/urlparser"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"net/url"
	"strings"
)

var _ aliasapi.AliasAPIServer = (*Controller)(nil)

type aliasService interface {
	Create(ctx context.Context, requests []domain.CreateRequest) ([]domain.Alias, error)
	FindOriginalURL(ctx context.Context, key string) (*domain.Alias, error)
	Use(ctx context.Context, alias *domain.Alias) (*domain.Alias, error)
	Remove(ctx context.Context, key string) error
}

type Controller struct {
	aliasapi.UnimplementedAliasAPIServer
	address string
	service aliasService
}

func NewController(service aliasService, address string) *Controller {
	return &Controller{service: service, address: address}
}

func (c *Controller) Create(ctx context.Context, data *aliasapi.CreateRequest) (*aliasapi.CreateResponse, error) {
	createRequests := make([]domain.CreateRequest, len(data.Urls))
	isPermanent := data.MaxUsageCount == nil
	triesLeft := data.GetMaxUsageCount()

	for index, urlString := range data.Urls {

		validURL, err := url.Parse(urlString)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid url fmt")
		}

		createRequests[index] = domain.CreateRequest{
			Params: domain.TTLParams{
				TriesLeft:   int(triesLeft),
				IsPermanent: isPermanent,
			},
			URL: validURL,
		}
	}

	answer, err := c.service.Create(ctx, createRequests)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	aliases := make([]string, len(answer))
	for index, alias := range answer {
		aliases[index] = fmt.Sprintf("%s/%s", c.address, alias.Key)
	}
	response := &aliasapi.CreateResponse{Urls: aliases}
	return response, nil
}

func (c *Controller) Remove(ctx context.Context, data *aliasapi.KeyRequest) (*emptypb.Empty, error) {

	if err := c.service.Remove(ctx, data.Key); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return nil, nil
}

func (c *Controller) FindOriginalURL(ctx context.Context, data *aliasapi.KeyRequest) (*aliasapi.FindResponse, error) {
	alias, err := c.service.FindOriginalURL(ctx, data.Key)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &aliasapi.FindResponse{Url: fmt.Sprintf("%s/%s", c.address, alias.Key)}, nil
}

func (c *Controller) HealthCheck(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, nil
}

func (c *Controller) ProcessMessage(ctx context.Context, data *aliasapi.ProcessMessageRequest) (*aliasapi.ProcessMessageResponse, error) {
	urls, err := urlparser.FindURLs(data.Message)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	request := &aliasapi.CreateRequest{
		Urls:          urls,
		MaxUsageCount: nil,
	}

	if len(request.Urls) == 0 {
		// message without urls
		return &aliasapi.ProcessMessageResponse{Message: data.Message}, nil
	}

	response, err := c.Create(ctx, request)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	result := data.Message

	for index, singeUrl := range urls {
		result = strings.Replace(result, singeUrl, response.Urls[index], 1)
	}

	return &aliasapi.ProcessMessageResponse{Message: result}, nil
}
