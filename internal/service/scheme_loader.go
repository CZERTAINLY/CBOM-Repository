package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func schemaLoaderFunc(httpClient *http.Client, timeout time.Duration) func(string) (io.ReadCloser, error) {
	return func(url string) (io.ReadCloser, error) {

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("`http.NewRequestWithContext()` failed: %w", err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http client `Do()` failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("received non-200 status code: %s", resp.Status)
		}

		return resp.Body, nil
	}
}
