package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/oas"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
	jss "github.com/kaptinlin/jsonschema"
)

const (
	httpClientTimeout = time.Second * 30
	jsonBOMSchemaV1_6 = "https://raw.githubusercontent.com/CycloneDX/specification/refs/heads/master/schema/bom-1.6.schema.json"
)

type Service struct {
	oas.UnimplementedHandler
	httpClient *http.Client
	store      store.Store
	jsonSchema *jss.Schema
}

func New(store store.Store) (Service, error) {
	httpClient := &http.Client{
		Timeout: httpClientTimeout, // safeguard if context aware method is not used for requests
	}

	compiler := jss.NewCompiler()
	compiler.RegisterLoader(jsonBOMSchemaV1_6, schemaLoaderFunc(httpClient, 10*time.Second))
	schema, err := compiler.GetSchema(jsonBOMSchemaV1_6)
	if err != nil {
		return Service{}, fmt.Errorf("json schema compiler `GetSchema()` failed: %w", err)
	}

	return Service{
		httpClient: httpClient,
		jsonSchema: schema,
		store:      store,
	}, nil
}

func (s Service) NewError(_ context.Context, err error) *oas.ErrorStatusCode {
	return &oas.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response: oas.Error{
			Message: err.Error(),
		},
	}
}

func (s Service) GetBOMByUrn(ctx context.Context, params oas.GetBOMByUrnParams) (oas.GetBOMByUrnRes, error) {
	var rt oas.GetBOMByUrnOKApplicationJSON

	if !params.Version.IsSet() {
		versions, err := s.store.GetObjectVersions(ctx, params.Urn)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return &oas.GetBOMByUrnNotFound{
				Message: fmt.Sprintf("CBOM does not exist, serial number %q.", params.Urn),
			}, nil

		case err != nil:
			return nil, err
		}

		params.Version.SetTo(versions[len(versions)-1])
	}

	b, err := s.store.GetObject(ctx, fmt.Sprintf("%s-%d", params.Urn, params.Version.Value))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return &oas.GetBOMByUrnNotFound{
			Message: fmt.Sprintf("CBOM does not exist, serial number %q, version %d.", params.Urn, params.Version.Value),
		}, nil

	case err != nil:
		return nil, err
	}

	rt = oas.GetBOMByUrnOKApplicationJSON(b)
	return &rt, nil
}

func (s Service) UploadBOM(ctx context.Context, req oas.UploadBOMReq) (oas.UploadBOMRes, error) {

	b, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, err
	}

	vres := s.jsonSchema.Validate(b)
	if !vres.IsValid() {
		return &oas.UploadBOMBadRequest{
			Message: fmt.Sprintf("Uploaded JSON does not conform to the following schema: %s",
				jsonBOMSchemaV1_6),
		}, nil
	}

	var cbom cdx.BOM
	decoder := cdx.NewBOMDecoder(bytes.NewReader(b), cdx.BOMFileFormatJSON)
	if err := decoder.Decode(&cbom); err != nil {
		return nil, err
	}

	if err := uploadInputChecks(cbom); err != nil {
		return &oas.UploadBOMBadRequest{
			Message: fmt.Sprintf("Upload input check failed: %s.", err),
		}, nil
	}

	if cbom.SerialNumber == "" {
		// serial number is missing, so we're going to generate a unique new one,
		// that means this will be version 1, even if something else was set
		cbom.Version = 1

		for {
			// generate a new urn and make sure we don't conflict with an exsiting one
			cbom.SerialNumber = fmt.Sprintf("urn:uuid:%s", uuid.NewString())
			exists, err := s.store.KeyExists(ctx, uploadKey(cbom.SerialNumber, cbom.Version))
			if err != nil {
				return nil, err
			}
			if !exists {
				break
			}
		}
	} else {
		// serial number of the CBOM is valid, let's make sure it doesn't exist already
		exists, err := s.store.KeyExists(ctx, uploadKey(cbom.SerialNumber, cbom.Version))
		if err != nil {
			return nil, err
		}
		if exists {
			return &oas.UploadBOMConflict{
				Message: fmt.Sprintf("CBOM already exists, serial number %q, version %q.",
					cbom.SerialNumber, cbom.Version),
			}, nil
		}
	}
	meta := store.Metadata{
		Timestamp: time.Now().UTC(),
		Version:   cbom.Version,
	}

	if err := s.store.Upload(ctx, uploadKey(cbom.SerialNumber, cbom.Version), meta, b); err != nil {
		return nil, err
	}

	return &oas.BOMCreateResponse{
		SerialNumber: cbom.SerialNumber,
		Version:      cbom.Version,
	}, nil
}

// uploadInputChecks returns error in case CBOM fails any of the input checks,
// nil otherwise.
func uploadInputChecks(cbom cdx.BOM) error {
	if cbom.SpecVersion != cdx.SpecVersion1_6 {
		return fmt.Errorf("required version %s", cdx.SpecVersion1_6)
	}
	if cbom.BOMFormat != cdx.BOMFormat {
		return fmt.Errorf("required format %s", cdx.BOMFormat)
	}
	// if the serial number is set, it must be a valid URN conforming to RFC 4122
	if cbom.SerialNumber != "" && !uploadValidateURN(cbom.SerialNumber) {
		return fmt.Errorf("serial number not valid")
	}
	return nil
}

// uploadValidateURN returns true if `urn` is a valid URN conforming to RFC-4122.
// URN format is defined as `urn:<NID>:<NSS>`
// where:
//   - <NID> means Namespace Identifier. For RFC-4122 this means exactly "uuid" string.
//   - <NSS> means Namespace Specific String. For RFC-4122 this means a valid UUID.
func uploadValidateURN(urn string) bool {
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

func uploadKey(urn string, version int) string {
	return fmt.Sprintf("%s-%d", urn, version)
}
