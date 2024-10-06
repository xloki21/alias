package client

import (
	"fmt"
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
		return nil, fmt.Errorf("could not create client: %w", err)
	}
	return &Client{Api: aliasapi.NewAliasAPIClient(cc)}, nil
}
