package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/CZERTAINLY/CBOM-Repository/internal/env"
	internalHttp "github.com/CZERTAINLY/CBOM-Repository/internal/http"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {

	// get configuration from environment variables
	cfg, err := env.New()
	if err != nil {
		panic(err)
	}

	// create s3 client
	s3cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	s3cfg.BaseEndpoint = aws.String(cfg.MinioEndpoint)
	var optFns []func(o *s3.Options)
	optFns = append(optFns, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	s3client := s3.NewFromConfig(s3cfg, optFns...)

	store := store.New(s3client, cfg.MinioBucket)
	svc := service.New(store)

	// start http server
	httpHandler := internalHttp.New(svc)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: internalHttp.Router(httpHandler),
	}

	fmt.Println("Starting http server.")

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		panic(err)
	}
}
