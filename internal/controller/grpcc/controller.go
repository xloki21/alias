package grpcc

import (
	"context"
	"errors"
	"fmt"
	"github.com/bufbuild/protovalidate-go"
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
	FindAlias(ctx context.Context, key string) (*domain.Alias, error)
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

	checker, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if err := checker.Validate(data); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	for index, item := range data.Urls {
		validURL, err := url.Parse(item.Url)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid url fmt")
		}

		createRequests[index] = domain.CreateRequest{
			Params: domain.TTLParams{
				TriesLeft:   item.GetMaxUsageCount(),
				IsPermanent: item.GetMaxUsageCount() == 0,
			},
			URL: validURL,
		}
	}

	answer, err := c.service.Create(ctx, createRequests)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, domain.ErrAliasCreationFailed.Error())
	}
	aliases := make([]string, len(answer))
	for index, alias := range answer {
		aliases[index] = fmt.Sprintf("%s/%s", c.address, alias.Key)
	}
	response := &aliasapi.CreateResponse{Urls: aliases}
	return response, nil
}

func (c *Controller) Remove(ctx context.Context, data *aliasapi.KeyRequest) (*emptypb.Empty, error) {
	checker, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if err := checker.Validate(data); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := c.service.Remove(ctx, data.Key); err != nil {
		if errors.Is(err, domain.ErrAliasNotFound) {
			return nil, status.Error(codes.NotFound, domain.ErrAliasNotFound.Error())
		}
		return nil, status.Error(codes.Internal, domain.ErrInternal.Error())
	}
	return nil, nil
}

func (c *Controller) FindAlias(ctx context.Context, data *aliasapi.KeyRequest) (*aliasapi.Alias, error) {
	checker, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if err := checker.Validate(data); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	alias, err := c.service.FindAlias(ctx, data.Key)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &aliasapi.Alias{
		Id:       alias.ID,
		Key:      alias.Key,
		Url:      alias.URL.String(),
		IsActive: alias.IsActive,
		Params: &aliasapi.AliasParams{
			IsPermanent: alias.Params.IsPermanent,
			TriesLeft:   alias.Params.TriesLeft,
		},
	}, nil
}

func (c *Controller) FindOriginalURL(ctx context.Context, data *aliasapi.KeyRequest) (*aliasapi.SingleURL, error) {
	checker, err := protovalidate.New()
	if err != nil {
		return nil, err
	}

	if err := checker.Validate(data); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	alias, err := c.service.FindAlias(ctx, data.Key)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &aliasapi.SingleURL{
		Url: alias.URL.String(),
	}, nil
}

func (c *Controller) ProcessMessage(ctx context.Context, data *aliasapi.ProcessMessageRequest) (*aliasapi.ProcessMessageResponse, error) {
	urls, err := urlparser.ExtractURLsFromText(data.Message)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if len(urls) == 0 {
		// message without urls, nothing to do, return original message
		return &aliasapi.ProcessMessageResponse{Message: data.Message}, nil
	}

	request := &aliasapi.CreateRequest{
		Urls: make([]*aliasapi.SingleURL, len(urls)),
	}
	for index := range urls {
		request.Urls[index] = &aliasapi.SingleURL{Url: urls[index]}
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
