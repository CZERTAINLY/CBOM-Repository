package env

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

const defaultPrefix = "APP"

type Config struct {
	// MinIO storage related
	StorageEndpoint string `envconfig:"APP_STORAGE_ENDPOINT" required:"true"`
	StorageBucket   string `envconfig:"APP_STORAGE_BUCKET" required:"true"`
}

func New() (Config, error) {
	var config Config
	err := envconfig.Process(defaultPrefix, &config)
	if err != nil {
		return Config{}, err
	}

	if strings.TrimSpace(config.StorageEndpoint) == "" {
		return Config{}, errors.New("environment variable `APP_MINIO_ENDPOINT` must not contain only whitespace characters")
	}

	if strings.TrimSpace(config.StorageBucket) == "" {
		return Config{}, errors.New("environment variable `APP_MINIO_BUCKET` must not contain only whitespace characters")
	}

	// although we don't hold the following AWS env variables directly, they are required by the `aws-sdk-go-v2`
	// library for connection to the MinIO storage
	for _, ev := range []string{"AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if strings.TrimSpace(os.Getenv(ev)) == "" {
			return Config{}, fmt.Errorf("environment variable `%q` not defined", ev)
		}
	}

	return config, nil
}
