package client

import (
	aliasapi "github.com/xloki21/alias/internal/gen/go/pbuf/alias"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	Api aliasapi.AliasAPIClient
}

func New(target string) (*Client, error) {
	cc, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{Api: aliasapi.NewAliasAPIClient(cc)}, nil
}
