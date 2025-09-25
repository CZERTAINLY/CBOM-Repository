package env_test

import (
	"testing"

	"github.com/CZERTAINLY/CBOM-Repository/internal/env"

	"github.com/stretchr/testify/require"
)

func TestNewFunc(t *testing.T) {
	testCases := map[string]struct {
		envVars map[string]string
		wantErr bool
		want    env.Config
	}{
		"success": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: false,
			want: env.Config{
				MinioEndpoint: "http://localhost:9000",
				MinioBucket:   "czertainly",
			},
		},
		"whitespaces-only-endpoint": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "  \t  \r   ",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"whitespaces-only-bucket": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "abcd",
				"APP_MINIO_BUCKET":      "\t  \r\n  ",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"whitespaces-only-aws-region": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "   \t ",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"whitespaces-only-aws-access-key": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "\t\t\r ",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"whitespaces-only-aws-secret": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "    ",
			},
			wantErr: true,
		},
		"endpoint-missing": {
			envVars: map[string]string{
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"bucket-missing": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"AWS_REGION":            "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"aws-region-missing": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_ACCESS_KEY_ID":     "minioadmin",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"aws-access-key-missing": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT":    "http://localhost:9000",
				"APP_MINIO_BUCKET":      "czertainly",
				"AWS_REGION":            "eu-west-1",
				"AWS_SECRET_ACCESS_KEY": "adminpassword",
			},
			wantErr: true,
		},
		"aws-secret-missing": {
			envVars: map[string]string{
				"APP_MINIO_ENDPOINT": "http://localhost:9000",
				"APP_MINIO_BUCKET":   "czertainly",
				"AWS_REGION":         "eu-west-1",
				"AWS_ACCESS_KEY_ID":  "minioadmin",
			},
			wantErr: true,
		},
		"empty environment": {
			envVars: map[string]string{},
			wantErr: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			setTestEnv(t, tc.envVars)

			cfg, err := env.New()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, cfg)
			}
		})
	}
}

// using `testing.Setenv()` we can prepare environment for each test case
// and have it automatically cleaned up after test
func setTestEnv(t *testing.T, envVars map[string]string) {
	t.Helper()

	for name, value := range envVars {
		t.Setenv(name, value)
	}
}
