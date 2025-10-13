package service

import (
	"bytes"
	"context"
	// "encoding/json"
	// "errors"
	"fmt"
	"io"
	"log/slog"
	// "maps"
	// "net/http"
	// "slices"
	"strings"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
)

type SBOMCreated struct {
	SerialNumber string `json:"serialNumber"`
	Version      int    `json:"version"`
}

func (s Service) UploadSBOM(ctx context.Context, rc io.ReadCloser, schemaVersion string) (SBOMCreated, error) {

	var buf bytes.Buffer
	tee := io.TeeReader(rc, &buf)
	defer rc.Close()

	ctx = log.ContextAttrs(ctx, slog.String("declared-sbom-schema-version", schemaVersion))

	var sbom cdx.BOM
	decoder := cdx.NewBOMDecoder(tee, cdx.BOMFileFormatJSON)
	if err := decoder.Decode(&sbom); err != nil {
		slog.ErrorContext(ctx, "`cdx.Decode()` failed.", slog.String("error", err.Error()))
		return SBOMCreated{}, err
	}

	if err := uploadInputChecks(sbom, schemaVersion); err != nil {
		return SBOMCreated{}, fmt.Errorf("%w: %s", ErrValidation, err)
	}

	jsonSchema, ok := s.jsonSchemas[schemaVersion]
	if !ok {
		// this shouldn't happen, if http handler correctly checks against `VersionSupported()`
		return SBOMCreated{}, fmt.Errorf("schema validator missing for version %s", schemaVersion)
	}

	res := jsonSchema.Validate(buf.Bytes())
	if !res.IsValid() {
		return SBOMCreated{}, fmt.Errorf("%w: does not conform to the declared schema", ErrValidation)
	}

	switch {
	case sbom.SerialNumber == "":
		slog.DebugContext(ctx, "SBOM does not have serial number specified - generating a new one.")
		// serial number is missing, so we're going to generate a unique new one,
		// that means this will be version 1, even if something else was set
		sbom.Version = 1

		for {
			// generate a new urn and make sure we don't conflict with an exsiting one
			sbom.SerialNumber = fmt.Sprintf("urn:uuid:%s", uuid.NewString())
			exists, err := s.store.KeyExists(ctx, uploadKey(sbom.SerialNumber, sbom.Version))
			if err != nil {
				return SBOMCreated{}, err
			}
			if !exists {
				break
			}
		}
		ctx = log.ContextAttrs(ctx, slog.Group(
			"service-layer",
			slog.String("new-serial-number", sbom.SerialNumber)),
		)
		slog.DebugContext(ctx, "New serial number generated.")

		// store the original unchanged SBOM
		metaOriginal := store.Metadata{
			Timestamp: time.Now().UTC(),
			Version:   "original",
		}
		if err := s.store.Upload(ctx, uploadKeyOriginal(sbom.SerialNumber), metaOriginal, buf.Bytes()); err != nil {
			return SBOMCreated{}, err
		}
		slog.DebugContext(ctx, "Stored original SBOM")

		// store the modified SBOM with serialNumber and version set
		meta := store.Metadata{
			Timestamp: time.Now().UTC(),
			Version:   fmt.Sprintf("%d", sbom.Version),
		}

		var modifiedBuf bytes.Buffer
		encoder := cdx.NewBOMEncoder(&modifiedBuf, cdx.BOMFileFormatJSON)
		if err := encoder.Encode(&sbom); err != nil {
			slog.ErrorContext(ctx, "`cdx.Encode()` failed.", slog.String("error", err.Error()))
			return SBOMCreated{}, err
		}

		if err := s.store.Upload(ctx, uploadKey(sbom.SerialNumber, sbom.Version), meta, modifiedBuf.Bytes()); err != nil {
			return SBOMCreated{}, err
		}
		slog.DebugContext(ctx, "Stored modified version")

	case sbom.Version < 1:
		slog.DebugContext(ctx, "SBOM has only serial number specified - fetching the latest version")
		versions, hasOriginal, err := s.store.GetObjectVersions(ctx, sbom.SerialNumber)
		if err != nil {
			return SBOMCreated{}, err
		}
		sbom.Version = versions[len(versions)-1] + 1
		ctx = log.ContextAttrs(ctx, slog.Group(
			"service-layer",
			slog.Int("new-version", sbom.Version),
			slog.Any("all-versions", versions),
			slog.Bool("has-original", hasOriginal),
		),
		)
		slog.DebugContext(ctx, "New version assigned to SBOM.")

		meta := store.Metadata{
			Timestamp: time.Now().UTC(),
			Version:   fmt.Sprintf("%d", sbom.Version),
		}

		var modifiedBuf bytes.Buffer
		encoder := cdx.NewBOMEncoder(&modifiedBuf, cdx.BOMFileFormatJSON)
		if err = encoder.Encode(&sbom); err != nil {
			return SBOMCreated{}, err
		}

		if err := s.store.Upload(ctx, uploadKey(sbom.SerialNumber, sbom.Version), meta, modifiedBuf.Bytes()); err != nil {
			return SBOMCreated{}, err
		}
		slog.DebugContext(ctx, "Stored modified version")

	default:
		slog.DebugContext(ctx, "SBOM has serial number and version specified.")
		// serial number of the SBOM is valid, version is set
		// let's make sure it doesn't exist already
		exists, err := s.store.KeyExists(ctx, uploadKey(sbom.SerialNumber, sbom.Version))
		if err != nil {
			return SBOMCreated{}, err
		}
		if exists {
			return SBOMCreated{
				SerialNumber: sbom.SerialNumber,
				Version:      sbom.Version,
			}, ErrAlreadyExists
		}

		meta := store.Metadata{
			Timestamp: time.Now().UTC(),
			Version:   fmt.Sprintf("%d", sbom.Version),
		}

		if err := s.store.Upload(ctx, uploadKey(sbom.SerialNumber, sbom.Version), meta, buf.Bytes()); err != nil {
			return SBOMCreated{}, err
		}
		slog.DebugContext(ctx, "Stored original SBOM")
	}

	return SBOMCreated{
		SerialNumber: sbom.SerialNumber,
		Version:      sbom.Version,
	}, nil
}

// uploadInputChecks returns error in case SBOM fails any of the input checks,
// nil otherwise.
func uploadInputChecks(sbom cdx.BOM, expectedVersion string) error {
	if sbom.BOMFormat != cdx.BOMFormat {
		return fmt.Errorf("required format %s", cdx.BOMFormat)
	}
	// if the serial number is set, it must be a valid URN conforming to RFC 4122
	if sbom.SerialNumber != "" && !uploadValidateURN(sbom.SerialNumber) {
		return fmt.Errorf("serial number not valid")
	}

	cdxVersion, err := knownCdxVersion(expectedVersion)
	if err != nil {
		return err
	}
	if sbom.SpecVersion != cdxVersion {
		return fmt.Errorf("required version %s", expectedVersion)
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

func uploadKeyOriginal(urn string) string {
	return fmt.Sprintf("%s-original", urn)
}

func knownCdxVersion(v string) (cdx.SpecVersion, error) {
	switch v {
	case "1.0":
		return cdx.SpecVersion1_0, nil
	case "1.1":
		return cdx.SpecVersion1_1, nil
	case "1.2":
		return cdx.SpecVersion1_2, nil
	case "1.3":
		return cdx.SpecVersion1_3, nil
	case "1.4":
		return cdx.SpecVersion1_4, nil
	case "1.5":
		return cdx.SpecVersion1_5, nil
	case "1.6":
		return cdx.SpecVersion1_6, nil
	default:
		return -1, fmt.Errorf("unknown cyclonedx sbom version %s", v)
	}
}
