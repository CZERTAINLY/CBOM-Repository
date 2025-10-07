package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	MetaTimestampKey   = "timestamp"
	MetaContentTypeKey = "content-type"
	MetaVersionKey     = "version"
)

type S3Contract interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type S3Manager interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type Config struct {
	Region       string `envconfig:"APP_S3_REGION" required:"true"`
	Endpoint     string `envconfig:"APP_S3_ENDPOINT"`
	Bucket       string `envconfig:"APP_S3_BUCKET" required:"true"`
	AccessKey    string `envconfig:"APP_S3_ACCESS_KEY" required:"true"`
	SecretKey    string `envconfig:"APP_S3_SECRET_KEY" required:"true"`
	UsePathStyle bool   `envconfig:"APP_S3_USE_PATH_STYLE" default:"true"`
}

type Store struct {
	cfg       Config
	s3Client  S3Contract
	s3Manager S3Manager
}

type Metadata struct {
	Timestamp time.Time
	Version   int
}

func (m Metadata) Map() map[string]string {
	return map[string]string{
		MetaTimestampKey: fmt.Sprintf("%d", m.Timestamp.Unix()),
		MetaVersionKey:   fmt.Sprintf("%d", m.Version),
	}
}

func New(cfg Config, s3Client S3Contract, s3Manager S3Manager) Store {
	s := Store{
		cfg:       cfg,
		s3Client:  s3Client,
		s3Manager: s3Manager,
	}

	return s
}

func (s Store) GetObjectVersions(ctx context.Context, urn string) ([]int, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.Bucket),
		Prefix: aws.String(urn),
	}

	var err error
	var output *s3.ListObjectsV2Output
	var objects []types.Object

	objectPaginator := s3.NewListObjectsV2Paginator(s.s3Client, input)
	for objectPaginator.HasMorePages() {
		if output, err = objectPaginator.NextPage(ctx); err != nil {
			return nil, err
		}
		objects = append(objects, output.Contents...)
	}

	if len(objects) == 0 {
		return nil, ErrNotFound
	}

	// post process just the versions
	var res []int
	for _, cpy := range objects {
		after, found := strings.CutPrefix(*cpy.Key, fmt.Sprintf("%s-", urn))
		if !found {
			return nil, errors.New("internal error")
		}
		ver, err := strconv.Atoi(after)
		if err != nil {
			return nil, errors.New("internal error")
		}
		res = append(res, ver)
	}
	sort.Ints(res)
	return res, nil
}

func (s Store) GetObject(ctx context.Context, key string) ([]byte, error) {
	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})

	var nsk *types.NoSuchKey
	var nf *types.NotFound

	switch {
	case errors.As(err, &nsk) || errors.As(err, &nf):
		return nil, ErrNotFound

	case err != nil:
		return nil, err
	}

	defer result.Body.Close()

	b, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (s Store) KeyExists(ctx context.Context, key string) (bool, error) {
	_, err := s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}

	var nsk *types.NoSuchKey
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return false, nil
	}

	return false, err
}

func (s Store) Upload(ctx context.Context, key string, meta Metadata, contents []byte) error {

	input := &s3.PutObjectInput{
		Bucket:            aws.String(s.cfg.Bucket),
		Key:               aws.String(key),
		Body:              bytes.NewReader(contents),
		Metadata:          meta.Map(),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ContentType:       aws.String("application/json"),
	}
	_, err := s.s3Manager.Upload(ctx, input)
	if err != nil {
		return err
	}

	return nil
}
