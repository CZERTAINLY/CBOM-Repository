package service

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	jss "github.com/kaptinlin/jsonschema"
)

var (
	ErrValidation    = errors.New("validation failed")
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

//go:embed schemas
var schemas embed.FS

var versionToEmbeddedFileMapping = map[string]string{
	"1.6": "schemas/bom-1.6.schema.json",
}

type SupportedVersions []string

// Expected format "<version-string^1>, <version-string^2>, ..."
// No duplicates allowed, at least one version has to be defined
func (s *SupportedVersions) Decode(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is an empty string")
	}
	v := map[string]bool{}
	items := strings.Split(value, ",")

	for _, cpy := range items {
		trimmed := strings.TrimSpace(cpy)
		if trimmed == "" {
			continue
		}
		if _, ok := v[trimmed]; ok {
			return errors.New("duplicate value")
		}
		v[trimmed] = true
	}
	if len(v) == 0 {
		return errors.New("there are only empty values")
	}

	*s = slices.Sorted(maps.Keys(v))
	return nil
}

type Config struct {
	Versions SupportedVersions `envconfig:"APP_SUPPORTED_VERSIONS" default:"1.6"`
}

type Service struct {
	cfg         Config
	store       store.Store
	jsonSchemas map[string]*jss.Schema
}

func New(cfg Config, store store.Store) (Service, error) {

	jsonSchemas := make(map[string]*jss.Schema, len(cfg.Versions))
	for _, version := range cfg.Versions {
		filename, ok := versionToEmbeddedFileMapping[version]
		if !ok {
			return Service{}, fmt.Errorf("missing mapping of embedded file for version %q", version)
		}
		b, err := schemas.ReadFile(filename)
		if err != nil {
			return Service{}, fmt.Errorf("failed to read embedded file %s: %w", filename, err)
		}

		compiler := jss.NewCompiler()
		schema, err := compiler.Compile(b)
		if err != nil {
			return Service{}, fmt.Errorf("failed to compile schema: %w", err)
		}
		jsonSchemas[version] = schema
	}

	return Service{
		cfg:         cfg,
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

func (s Service) GetBOMByUrn(ctx context.Context, urn, version string) (map[string]interface{}, error) {
	ctx = log.ContextAttrs(ctx, slog.Group(
		"service-layer",
		slog.String("urn", urn),
		slog.String("version", version),
	))

	if strings.TrimSpace(version) == "" {
		slog.DebugContext(ctx, "Version is empty, calling `store.GetObjectVersion()` to obtain the latest BOM version stored.")
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

	var bomMap map[string]interface{}
	if err := json.Unmarshal(b, &bomMap); err != nil {
		slog.ErrorContext(ctx, "`json.Unmarshal()` failed.", slog.String("error", err.Error()))
		return nil, err
	}

	return bomMap, nil
}
