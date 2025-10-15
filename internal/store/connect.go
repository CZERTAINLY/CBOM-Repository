package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func ConnectS3(ctx context.Context, cfg Config) (*s3.Client, *manager.Uploader, error) {
	s3cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	if cfg.Endpoint != "" {
		s3cfg.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	var optFns []func(o *s3.Options)
	optFns = append(optFns, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	s3Client := s3.NewFromConfig(s3cfg, optFns...)

	// there is no ping() in the `aws-sdk-go-v2`, so we'll do connection check
	// with a HeadBucket operation
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, nil, err
	}

	return s3Client, manager.NewUploader(s3Client), nil
}
