package store

import (
	"bytes"
	"context"
	"fmt"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Contract interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type Store struct {
	s3Client S3Contract
	bucket   string
}

func New(s3Client S3Contract, bucket string) Store {
	return Store{
		s3Client: s3Client,
		bucket:   bucket,
	}
}

func (s Store) Upload(ctx context.Context, cbom cdx.BOM) error {

	var buf bytes.Buffer
	err := cdx.NewBOMEncoder(&buf, cdx.BOMFileFormatJSON).Encode(&cbom)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(buf.Bytes())

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(uploadKey(cbom)),
		Body:   reader,
	})
	if err != nil {
		return err
	}

	return nil
}

func uploadKey(cbom cdx.BOM) string {
	return fmt.Sprintf("%s-%d", cbom.SerialNumber, cbom.Version)
}
