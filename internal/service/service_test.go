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

	_, err := service.New(store.New(store.Config{Bucket: "something"}, s3Mock, s3Manager))
	require.NoError(t, err)
}
