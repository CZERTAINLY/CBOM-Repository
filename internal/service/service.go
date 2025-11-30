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

	"github.com/google/uuid"
	jss "github.com/kaptinlin/jsonschema"
)

var (
	ErrValidation    = errors.New("validation failed")
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

//go:embed schemas
var schemas embed.FS

// Please note: When you want to add a new schema version, please first
// add the schema file into the `schemas` subdirectory in `interna/service`
// and then extend this variable with the mapping.
var versionToEmbeddedFileMapping = map[string]string{
	"1.6": "schemas/bom-1.6.schema.json",
}

type Service struct {
	store       store.Store
	jsonSchemas map[string]*jss.Schema
}

func New(store store.Store) (Service, error) {

	jsonSchemas := make(map[string]*jss.Schema)
	for version, filename := range versionToEmbeddedFileMapping {
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

	ctx = log.ContextAttrs(ctx, slog.Int64("timestamp", ts))
	slog.DebugContext(ctx, "Calling `store.Search()`.")

	r, err := s.store.Search(ctx, ts)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "`store.Search()` finished.",
		slog.Int("count", len(r)),
		slog.String("value", strings.Join(r, ",")),
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
	ctx = log.ContextAttrs(ctx,
		slog.String("urn", urn),
		slog.String("version", version),
	)

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
		slog.DebugContext(ctx, "Latest version selected.", slog.Group("getObjectVersionsResult",
			slog.Any("all-versions", versions),
			slog.String("selected-version", version),
			slog.Bool("has-original", hasOriginal),
		),
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

// URNValid returns true if `urn` is a valid URN conforming to RFC-4122.
// URN format is defined as `urn:<NID>:<NSS>`
// where:
//   - <NID> means Namespace Identifier. For RFC-4122 this means exactly "uuid" string.
//   - <NSS> means Namespace Specific String. For RFC-4122 this means a valid UUID.
func URNValid(urn string) bool {
	subs := strings.Split(urn, ":")
	if len(subs) != 3 {
		return false
	}
	if subs[0] != "urn" || subs[1] != "uuid" {
		return false
	}
	if _, err := uuid.Parse(subs[2]); err != nil {
		return false
	}
	return true
}
