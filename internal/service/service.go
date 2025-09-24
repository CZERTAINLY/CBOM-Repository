package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
)

type Service struct {
	store store.Store
}

func New(store store.Store) Service {
	return Service{
		store: store,
	}
}

func (s Service) Upload(ctx context.Context, reader io.Reader) error {

	var cbom cdx.BOM
	decoder := cdx.NewBOMDecoder(reader, cdx.BOMFileFormatJSON)
	if err := decoder.Decode(&cbom); err != nil {
		return err
	}

	if err := uploadInputChecks(cbom); err != nil {
		return err
	}

	if err := s.store.Upload(ctx, cbom); err != nil {
		return err
	}

	return nil
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
