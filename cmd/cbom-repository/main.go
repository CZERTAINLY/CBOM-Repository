package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/CZERTAINLY/CBOM-Repository/internal/env"
	"github.com/CZERTAINLY/CBOM-Repository/internal/oas"
	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"
)

func main() {

	// get configuration from environment variables
	cfg, err := env.New()
	if err != nil {
		panic(err)
	}

	s3Client, s3Uploader, err := store.ConnectS3(context.Background(), cfg.Store)
	if err != nil {
		panic(err)
	}

	store := store.New(cfg.Store, s3Client, s3Uploader)
	svc, err := service.New(store)
	if err != nil {
		panic(err)
	}

	srv, err := oas.NewServer(svc)
	if err != nil {
		panic(err)
	}

	fmt.Println("Starting http server.")

	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.HttpPort), srv); err != http.ErrServerClosed {
		panic(err)
	}
}
