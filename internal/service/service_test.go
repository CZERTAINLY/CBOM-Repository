package service_test

import (
	"testing"

	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	mockS3 "github.com/CZERTAINLY/CBOM-Repository/internal/store/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const jsonBOMSchemaV1_6 = "https://raw.githubusercontent.com/CycloneDX/specification/refs/heads/master/schema/bom-1.6.schema.json"

func TestNewFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s3Mock := mockS3.NewMockS3Contract(ctrl)
	s3Manager := mockS3.NewMockS3Manager(ctrl)

	svc, err := service.New(
		service.Config{
			Versions: map[string]string{
				"1.6": jsonBOMSchemaV1_6,
			},
		},
		store.New(store.Config{Bucket: "something"}, s3Mock, s3Manager),
	)
	require.NoError(t, err)
	require.True(t, svc.VersionSupported("1.6"))
	require.False(t, svc.VersionSupported("1.4"))
	require.Equal(t, []string{"1.6"}, svc.SupportedVersion())
}

func TestSupportedVersionDecode(t *testing.T) {

	testCases := map[string]struct {
		input    string
		wantErr  bool
		expected map[string]string
	}{
		"empty": {
			input:   "",
			wantErr: true,
		},
		"success": {
			input:   "1.6 = https://some.url.com/schema?version=1.6, 1.7= https://uri2,k3=uri3",
			wantErr: false,
			expected: map[string]string{
				"1.7": "https://uri2",
				"k3":  "uri3",
				"1.6": "https://some.url.com/schema?version=1.6",
			},
		},
		"fail missing delimiter": {
			input:   "k1=uri1, k2= uri2,k3 uri3, k4=uri4",
			wantErr: true,
		},
		"extra comma at the end": {
			input:   "k1=uri1, k2= uri2,k3 uri3,",
			wantErr: true,
		},
		"extra comma in the middle": {
			input:   "k1=uri1, k2=, uri2,k3 uri3,",
			wantErr: true,
		},
		"duplicate key": {
			input:   "k1=uri1,k2=uri2,k3=uri3,k1=uri4,k5=uri5",
			wantErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := service.SupportedVersions{}

			err := s.Decode(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, tc.expected, s)
			}
		})
	}
}
