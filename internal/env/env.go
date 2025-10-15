package env

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	"github.com/kelseyhightower/envconfig"
)

const defaultPrefix = "APP"

type Config struct {
	Store    store.Config
	HttpPort int `envconfig:"APP_HTTP_PORT" default:"8080"`
	Service  service.Config
	LogLevel slog.Level `envconfig:"APP_LOG_LEVEL" default:"INFO"`
}

func New() (Config, error) {
	var config Config
	err := envconfig.Process(defaultPrefix, &config)
	if err != nil {
		return Config{}, err
	}

	if strings.TrimSpace(config.Store.Region) == "" {
		return Config{}, errors.New("environment variable `APP_S3_REGION` must not contain whitespace characters only")
	}

	if strings.TrimSpace(config.Store.Bucket) == "" {
		return Config{}, errors.New("environment variable `APP_S3_BUCKET` must not contain whitespace characters only")
	}

	if strings.TrimSpace(config.Store.AccessKey) == "" {
		return Config{}, errors.New("environment variable `APP_S3_ACCESS_KEY` must not contain whitespace characters only")
	}

	if strings.TrimSpace(config.Store.SecretKey) == "" {
		return Config{}, errors.New("environment variable `APP_S3_SECRET_KEY` must not contain whitespace characters only")
	}

	return config, nil
}
