package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/log"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
)

type BOMCreated struct {
	SerialNumber string   `json:"serialNumber"`
	Version      int      `json:"version"`
	SimpleStats  BomStats `json:"stats"`
}

func (s Service) UploadBOM(ctx context.Context, rc io.ReadCloser, schemaVersion string) (BOMCreated, error) {

	var buf bytes.Buffer
	tee := io.TeeReader(rc, &buf)
	defer func() {
		_ = rc.Close()
	}()

	ctx = log.ContextAttrs(ctx, slog.String("declared-bom-schema-version", schemaVersion))

	var bom cdx.BOM
	decoder := cdx.NewBOMDecoder(tee, cdx.BOMFileFormatJSON)
	if err := decoder.Decode(&bom); err != nil {
		slog.ErrorContext(ctx, "`cdx.Decode()` failed.", slog.String("error", err.Error()))
		return BOMCreated{}, err
	}

	if err := uploadInputChecks(bom, schemaVersion); err != nil {
		return BOMCreated{}, fmt.Errorf("%w: %s", ErrValidation, err)
	}

	jsonSchema, ok := s.jsonSchemas[schemaVersion]
	if !ok {
		// this shouldn't happen, if http handler correctly checks against `VersionSupported()`
		return BOMCreated{}, fmt.Errorf("schema validator missing for version %s", schemaVersion)
	}

	res := jsonSchema.Validate(buf.Bytes())
	if !res.IsValid() {
		return BOMCreated{}, fmt.Errorf("%w: does not conform to the declared schema", ErrValidation)
	}

	bomStats := BOMStats(ctx, &bom)
	b, err := json.Marshal(bomStats)
	if err != nil {
		return BOMCreated{}, fmt.Errorf("`json.Marshal()` failed: %w", err)
	}

	var retVal BOMCreated
	var retErr error
	switch {
	case bom.SerialNumber == "":
		retVal, retErr = s.uploadCaseSNInvalid(ctx, bom, buf, string(b))

	case bom.Version < 1:
		retVal, retErr = s.uploadCaseSNValidVersionInvalid(ctx, bom, string(b))

	default:
		// serial number of the BOM is valid, version is set
		retVal, retErr = s.uploadCaseSNValidVersionValid(ctx, bom, buf, string(b))
	}
	if retErr == nil {
		retVal.SimpleStats = bomStats
	}
	return retVal, retErr
}

func (s Service) uploadCaseSNInvalid(ctx context.Context, bom cdx.BOM, orig bytes.Buffer, stats string) (BOMCreated, error) {
	slog.DebugContext(ctx, "BOM does not have serial number specified - generating a new one.")
	// serial number is missing, so we're going to generate a unique new one,
	// that means this will be version 1, even if something else was set
	bom.Version = 1

	for {
		// generate a new urn and make sure we don't conflict with an existing one
		bom.SerialNumber = fmt.Sprintf("urn:uuid:%s", uuid.NewString())
		exists, err := s.store.KeyExists(ctx, uploadKey(bom.SerialNumber, bom.Version))
		if err != nil {
			return BOMCreated{}, err
		}
		if !exists {
			break
		}
	}
	ctx = log.ContextAttrs(ctx, slog.String("new-serial-number", bom.SerialNumber))
	slog.DebugContext(ctx, "New serial number generated.")

	// store the original unchanged BOM
	metaOriginal := store.Metadata{
		Timestamp: time.Now().UTC(),
		Version:   "original",
		Stats:     stats,
	}
	if err := s.store.Upload(ctx, uploadKeyOriginal(bom.SerialNumber), metaOriginal, orig.Bytes()); err != nil {
		return BOMCreated{}, err
	}
	slog.DebugContext(ctx, "Stored original BOM.")

	// store the modified BOM with serialNumber and version set
	meta := store.Metadata{
		Timestamp: time.Now().UTC(),
		Version:   fmt.Sprintf("%d", bom.Version),
		Stats:     stats,
	}

	var modifiedBuf bytes.Buffer
	encoder := cdx.NewBOMEncoder(&modifiedBuf, cdx.BOMFileFormatJSON)
	if err := encoder.Encode(&bom); err != nil {
		slog.ErrorContext(ctx, "`cdx.Encode()` failed.", slog.String("error", err.Error()))
		return BOMCreated{}, err
	}

	if err := s.store.Upload(ctx, uploadKey(bom.SerialNumber, bom.Version), meta, modifiedBuf.Bytes()); err != nil {
		return BOMCreated{}, err
	}
	slog.DebugContext(ctx, "Stored modified version.")

	return BOMCreated{
		SerialNumber: bom.SerialNumber,
		Version:      bom.Version,
	}, nil
}

func (s Service) uploadCaseSNValidVersionInvalid(ctx context.Context, bom cdx.BOM, stats string) (BOMCreated, error) {
	slog.DebugContext(ctx, "BOM has only serial number specified - fetching the latest version")
	versions, hasOriginal, err := s.store.GetObjectVersions(ctx, bom.SerialNumber)
	switch {
	case errors.Is(err, store.ErrNotFound):
		bom.Version = 1
		slog.DebugContext(ctx, "First BOM with this SN, assigning Version '1'.")
	case err != nil:
		return BOMCreated{}, err
	default:
		bom.Version = versions[len(versions)-1] + 1
		slog.DebugContext(ctx, "New version assigned to BOM.",
			slog.Int("new-version", bom.Version),
			slog.Any("all-versions", versions),
			slog.Bool("has-original", hasOriginal),
		)
	}

	meta := store.Metadata{
		Timestamp: time.Now().UTC(),
		Version:   fmt.Sprintf("%d", bom.Version),
		Stats:     stats,
	}

	var modifiedBuf bytes.Buffer
	encoder := cdx.NewBOMEncoder(&modifiedBuf, cdx.BOMFileFormatJSON)
	if err = encoder.Encode(&bom); err != nil {
		return BOMCreated{}, err
	}

	if err := s.store.Upload(ctx, uploadKey(bom.SerialNumber, bom.Version), meta, modifiedBuf.Bytes()); err != nil {
		return BOMCreated{}, err
	}
	slog.DebugContext(ctx, "Stored modified BOM.")
	return BOMCreated{
		SerialNumber: bom.SerialNumber,
		Version:      bom.Version,
	}, nil
}

func (s Service) uploadCaseSNValidVersionValid(ctx context.Context, bom cdx.BOM, orig bytes.Buffer, stats string) (BOMCreated, error) {
	slog.DebugContext(ctx, "BOM has serial number and version specified.")
	// let's make sure it doesn't exist already
	exists, err := s.store.KeyExists(ctx, uploadKey(bom.SerialNumber, bom.Version))
	if err != nil {
		return BOMCreated{}, err
	}
	if exists {
		return BOMCreated{
			SerialNumber: bom.SerialNumber,
			Version:      bom.Version,
		}, ErrAlreadyExists
	}

	meta := store.Metadata{
		Timestamp: time.Now().UTC(),
		Version:   fmt.Sprintf("%d", bom.Version),
		Stats:     stats,
	}

	if err := s.store.Upload(ctx, uploadKey(bom.SerialNumber, bom.Version), meta, orig.Bytes()); err != nil {
		return BOMCreated{}, err
	}
	slog.DebugContext(ctx, "Stored original BOM")

	return BOMCreated{
		SerialNumber: bom.SerialNumber,
		Version:      bom.Version,
	}, nil
}

// uploadInputChecks returns error in case BOM fails any of the input checks,
// nil otherwise.
func uploadInputChecks(bom cdx.BOM, expectedVersion string) error {
	if bom.BOMFormat != cdx.BOMFormat {
		return fmt.Errorf("required format %s", cdx.BOMFormat)
	}
	// if the serial number is set, it must be a valid URN conforming to RFC 4122
	if bom.SerialNumber != "" && !URNValid(bom.SerialNumber) {
		return fmt.Errorf("serial number not valid")
	}

	cdxVersion, err := knownCdxVersion(expectedVersion)
	if err != nil {
		return err
	}
	if bom.SpecVersion != cdxVersion {
		return fmt.Errorf("required version %s", expectedVersion)
	}

	return nil
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
		return -1, fmt.Errorf("unknown cyclonedx bom version %s", v)
	}
}
