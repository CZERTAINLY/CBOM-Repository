package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/CZERTAINLY/CBOM-Repository/internal/store"
	mockS3 "github.com/CZERTAINLY/CBOM-Repository/internal/store/mock"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const bucketName = "bucket"

func TestStoreUpload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	okCBOM := cdx.BOM{
		BOMFormat:    cdx.BOMFormat,
		SpecVersion:  cdx.SpecVersion1_6,
		SerialNumber: "urn:uuid:5bd5a7c5-f5f0-40db-a216-d242abba1185",
		Version:      1,
		Metadata: &cdx.Metadata{
			Timestamp: time.Now().String(),
			Authors:   &[]cdx.OrganizationalContact{{Name: "John Doe", Email: "john@doe.com"}},
		},
	}

	tests := map[string]struct {
		cbom    cdx.BOM
		setup   func(mockCtrl *gomock.Controller, cbom cdx.BOM) store.Store
		wantErr bool
	}{
		"success": {
			cbom: okCBOM,
			setup: func(mockCtrl *gomock.Controller, cbom cdx.BOM) store.Store {
				s3Mock := mockS3.NewMockS3Contract(mockCtrl)
				s3Mock.EXPECT().PutObject(
					gomock.Any(),
					gomock.AssignableToTypeOf(&s3.PutObjectInput{}),
				).DoAndReturn(func(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					require.Equal(t, bucketName, *in.Bucket)
					require.Equal(t, fmt.Sprintf("%s-%d", cbom.SerialNumber, cbom.Version), *in.Key)
					return &s3.PutObjectOutput{}, nil
				})

				return store.New(s3Mock, bucketName)
			},
			wantErr: false,
		},
		"put object returns error": {
			cbom: okCBOM,
			setup: func(mockCtrl *gomock.Controller, cbom cdx.BOM) store.Store {
				s3Mock := mockS3.NewMockS3Contract(mockCtrl)
				s3Mock.EXPECT().PutObject(
					gomock.Any(),
					gomock.AssignableToTypeOf(&s3.PutObjectInput{}),
				).DoAndReturn(func(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					require.Equal(t, bucketName, *in.Bucket)
					require.Equal(t, fmt.Sprintf("%s-%d", cbom.SerialNumber, cbom.Version), *in.Key)
					return &s3.PutObjectOutput{}, errors.New("abc")
				})

				return store.New(s3Mock, bucketName)
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := tc.setup(ctrl, tc.cbom)
			err := s.Upload(context.Background(), tc.cbom)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

}
