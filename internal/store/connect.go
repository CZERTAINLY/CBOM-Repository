package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	manager "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ConnectS3 establishes a connection to an S3-compatible object storage service and returns
// an S3 client and uploader. It configures the client with the provided credentials, region,
// and optional endpoint settings. The function verifies connectivity by performing a HeadBucket
// operation on the specified bucket.
//
// Parameters:
//   - ctx: Context for cancellation, deadlines and additional slog fields.
//   - cfg: Configuration containing AWS credentials, region, bucket name, and optional endpoint settings
//
// Returns:
//   - *s3.Client: Configured S3 client for performing S3 operations
//   - *manager.Client: transfer manager.Client for efficient multi-part uploads
//   - error: Any error encountered during configuration or connection verification
func ConnectS3(ctx context.Context, cfg Config) (*s3.Client, *manager.Client, error) {
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

	return s3Client, manager.New(s3Client), nil
}
