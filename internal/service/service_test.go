package service_test

import (
	"testing"

	"github.com/CZERTAINLY/CBOM-Repository/internal/service"
	"github.com/CZERTAINLY/CBOM-Repository/internal/store"

	mockS3 "github.com/CZERTAINLY/CBOM-Repository/internal/store/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewFunc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s3Mock := mockS3.NewMockS3Contract(ctrl)
	s3Manager := mockS3.NewMockS3Manager(ctrl)

	svc, err := service.New(
		service.Config{
			Versions: []string{"1.6"},
		},
		store.New(store.Config{Bucket: "something"}, s3Mock, s3Manager),
	)
	require.NoError(t, err)
	require.True(t, svc.VersionSupported("1.6"))
	require.False(t, svc.VersionSupported("1.4"))
	require.Equal(t, []string{"1.6"}, svc.SupportedVersion())
}

func TestNewFuncNoMapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s3Mock := mockS3.NewMockS3Contract(ctrl)
	s3Manager := mockS3.NewMockS3Manager(ctrl)

	_, err := service.New(
		service.Config{
			Versions: []string{"abc"},
		},
		store.New(store.Config{Bucket: "something"}, s3Mock, s3Manager),
	)
	require.Error(t, err)
}

func TestSupportedVersionDecode(t *testing.T) {

	testCases := map[string]struct {
		input    string
		wantErr  bool
		expected []string
	}{
		"empty": {
			input:   "",
			wantErr: true,
		},
		"success": {
			input:    "1.6, 1.7",
			wantErr:  false,
			expected: []string{"1.6", "1.7"},
		},
		"success-2": {
			input:    "1.6",
			wantErr:  false,
			expected: []string{"1.6"},
		},
		"empty item is skipped": {
			input:    ", 1.6,,1.5, ,,",
			wantErr:  false,
			expected: []string{"1.5", "1.6"},
		},
		"duplicate values": {
			input:   ", 1.6,1.5, ,1.6, ,",
			wantErr: true,
		},
		"empty values only": {
			input:   ", ,,\t , ,\t\t ,,",
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
