package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/xloki21/alias/internal/client"
	aliasapi "github.com/xloki21/alias/internal/gen/go/pbuf/alias"
	"log"
	"os"
)

func main() {
	target := flag.String("address", "localhost:8081", "host:port")
	message := flag.String("message", "", "message to send")
	if len(os.Args) < 3 {
		flag.Usage()
	}
	flag.Parse()

	ctx := context.Background()
	aliasClient, err := client.New(*target)
	if err != nil {
		log.Fatal(err)
	}

	response, err := aliasClient.Api.ProcessMessage(ctx,
		&aliasapi.ProcessMessageRequest{
			Message: *message,
		})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response)

}
