package service

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/stretchr/testify/require"
)

func TestUploadInputChecks(t *testing.T) {
	testCases := map[string]struct {
		input   cdx.BOM
		wantErr bool
	}{
		"empty struct": {
			input:   cdx.BOM{},
			wantErr: true,
		},
		"success": {
			input: cdx.BOM{
				SpecVersion:  cdx.SpecVersion1_6,
				BOMFormat:    cdx.BOMFormat,
				SerialNumber: "urn:uuid:550e8400-e29b-11d4-a716-446655440000",
			},
			wantErr: false,
		},
		"bad serial": {
			input: cdx.BOM{
				SpecVersion:  cdx.SpecVersion1_6,
				BOMFormat:    cdx.BOMFormat,
				SerialNumber: "urn:isbn:0451450523",
			},
			wantErr: true,
		},
		"lower spec version": {
			input: cdx.BOM{
				SpecVersion:  cdx.SpecVersion1_4,
				BOMFormat:    cdx.BOMFormat,
				SerialNumber: "urn:uuid:550e8400-e29b-11d4-a716-446655440000",
			},
			wantErr: true,
		},
		"unexpected bom format": {
			input: cdx.BOM{
				SpecVersion:  cdx.SpecVersion1_6,
				BOMFormat:    "something",
				SerialNumber: "urn:uuid:550e8400-e29b-11d4-a716-446655440000",
			},
			wantErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := uploadInputChecks(tc.input, "1.6")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUploadValidateURN(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  bool
	}{
		"valid-v1-mac": {
			input: "urn:uuid:550e8400-e29b-11d4-a716-446655440000",
			want:  true,
		},
		"valid-v2-dce": {
			input: "urn:uuid:6ba7b810-9dad-21d1-80b4-00c04fd430c8",
			want:  true,
		},
		"valid-v3-md5": {
			input: "urn:uuid:c30c6b7b-107b-34df-b214-eb13f774fffa",
			want:  true,
		},
		"valid-v4-random": {
			input: "urn:uuid:9b2c51f2-6d3a-4c9a-8f3f-3a2c5f5a9c9d",
			want:  true,
		},
		"valid-v5-sha1": {
			input: "urn:uuid:2e93abd6-3a33-5e7d-b0c3-97c0d57b6d43",
			want:  true,
		},
		"valid-v6-time-based": {
			input: "urn:uuid:1ec9414c-232a-6b00-b3c8-9e8bde5ac4b8",
			want:  true,
		},
		"valid-v7-time-ordered": {
			input: "urn:uuid:019976ff-0e57-7044-8525-2a01f8e13736",
			want:  true,
		},
		"valid-v8-custom-format": {
			input: "urn:uuid:123e4567-e89b-89d3-a456-426614174000",
			want:  true,
		},
		"malformed-1": {
			input: "uuid:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-2": {
			input: ":uuid:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-3": {
			input: "xyz:uuid:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-4": {
			input: "urn::uuid:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-5": {
			input: "urn::ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-6": {
			input: "urn:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-7": {
			input: "urn:uuid::ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-8": {
			input: "urn:md5:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"malformed-9": {
			input: "uuid:urn:ecc69056-a50b-4c4c-9f25-fbb35af91f4d",
			want:  false,
		},
		"not-valid-uuid": {
			input: "urn:uuid:ecc69056-ax0b-4c4c-0f25-f0035af91f4d",
			want:  false,
		},
		"missing-uuid": {
			input: "urn:uuid:",
			want:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := uploadValidateURN(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
