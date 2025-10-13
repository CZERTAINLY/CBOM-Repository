package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	jss "github.com/kaptinlin/jsonschema"
)

const (
	httpClientTimeout = time.Second * 30
)

var (
	ErrValidation    = errors.New("validation failed")
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

type SupportedVersions map[string]string

// Expected "<version-string^1>=<schema-uri^1>, <version-string^2>=<schema-uri^2>, ..."
func (s *SupportedVersions) Decode(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is an empty string")
	}
	m := make(map[string]string)
	items := strings.Split(value, ",")

	for _, item := range items {
		before, after, found := strings.Cut(item, "=")
		if !found {
			return fmt.Errorf("invalid item: %s", item)
		}
		key := strings.TrimSpace(before)
		if _, ok := m[key]; ok {
			return fmt.Errorf("duplicate key: %s", key)
		}
		m[key] = strings.TrimSpace(after)
	}
	*s = m
	return nil
}

type Config struct {
	Versions SupportedVersions `envconfig:"APP_SUPPORTED_VERSIONS" required:"true"`
}

type Service struct {
	cfg         Config
	httpClient  *http.Client
	store       store.Store
	jsonSchemas map[string]*jss.Schema
}

func New(cfg Config, store store.Store) (Service, error) {
	httpClient := &http.Client{
		Timeout: httpClientTimeout, // safeguard if context aware method is not used for requests
	}

	jsonSchemas := make(map[string]*jss.Schema, len(cfg.Versions))
	for version, uri := range cfg.Versions {
		compiler := jss.NewCompiler()
		compiler.RegisterLoader(uri, schemaLoaderFunc(httpClient, 10*time.Second))
		schema, err := compiler.GetSchema(uri)
		if err != nil {
			return Service{}, fmt.Errorf("json schema compiler `GetSchema()` failed for URI %s: %w", uri, err)
		}
		jsonSchemas[version] = schema
	}

	return Service{
		cfg:         cfg,
		httpClient:  httpClient,
		jsonSchemas: jsonSchemas,
		store:       store,
	}, nil
}

func (s Service) SupportedVersion() []string {
	return slices.Sorted(maps.Keys(s.jsonSchemas))
}

func (s Service) VersionSupported(version string) bool {
	if _, ok := s.jsonSchemas[version]; ok {
		return true
	}
	return false
}

type SearchRes struct {
	URN     string `json:"serialNumber"`
	Version string `json:"version"`
}

func (s Service) Search(ctx context.Context, ts int64) ([]SearchRes, error) {
	res := []SearchRes{}

	ctx = log.ContextAttrs(ctx, slog.Group(
		"service-layer",
		slog.Int64("timestamp", ts),
	))

	slog.DebugContext(ctx, "Calling `store.Search()`.")

	r, err := s.store.Search(ctx, ts)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "`store.Search()` finished.",
		slog.Group(
			"response",
			slog.Int("count", len(r)),
			slog.String("value", strings.Join(r, ",")),
		),
	)

	for _, cpy := range r {
		idx := strings.LastIndex(cpy, "-")
		if idx == -1 {
			slog.ErrorContext(ctx, "Key does NOT adhere to the naming invariant.",
				slog.String("key", cpy), slog.String("expected-format", "urn:uuid:<uuid>-<version>"))
			return nil, errors.New("unexpected key returned from store")
		}
		res = append(res, SearchRes{
			URN:     cpy[:idx],
			Version: cpy[idx+1:],
		})
	}
	return res, nil
}

func (s Service) GetSBOMByUrn(ctx context.Context, urn, version string) (map[string]interface{}, error) {
	ctx = log.ContextAttrs(ctx, slog.Group(
		"service-layer",
		slog.String("urn", urn),
		slog.String("version", version),
	))

	if strings.TrimSpace(version) == "" {
		slog.DebugContext(ctx, "Version is empty, calling `store.GetObjectVersion()` to obtain the latest SBOM version stored.")
		versions, hasOriginal, err := s.store.GetObjectVersions(ctx, urn)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return nil, ErrNotFound

		case err != nil:
			return nil, err
		}

		version = fmt.Sprintf("%d", versions[len(versions)-1])
		slog.DebugContext(ctx, "Latest version selected.",
			slog.Any("all-versions", versions),
			slog.String("selected-version", version),
			slog.Bool("has-original", hasOriginal),
		)
		ctx = log.ContextAttrs(ctx, slog.String("selected-version", version))
	}

	slog.DebugContext(ctx, "Calling `store.GetObject()`.")
	b, err := s.store.GetObject(ctx, fmt.Sprintf("%s-%s", urn, version))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, ErrNotFound

	case err != nil:
		return nil, err
	}
	slog.DebugContext(ctx, "`store.GetObject()` finished.", slog.Int64("size", int64(len(b))))

	var sbomMap map[string]interface{}
	if err := json.Unmarshal(b, &sbomMap); err != nil {
		slog.ErrorContext(ctx, "`json.Unmarshal()` failed.", slog.String("error", err.Error()))
		return nil, err
	}

	return sbomMap, nil
}
