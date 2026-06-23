package baseten

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const responsePreviewByteLimit = 4096

type Client struct {
	apiKey     string
	baseURL    *url.URL
	httpClient *http.Client
}

type InstanceType struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	MemoryLimitMiB    int64   `json:"memory_limit_mib"`
	MillicpuLimit     int64   `json:"millicpu_limit"`
	GPUCount          int64   `json:"gpu_count"`
	GPUType           *string `json:"gpu_type"`
	GPUMemoryLimitMiB *int64  `json:"gpu_memory_limit_mib"`
}

type instanceTypesResponse struct {
	InstanceTypes []InstanceType `json:"instance_types"`
}

func NewClient(apiKey string, endpoint string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key")
	}

	if endpoint == "" {
		return nil, errors.New("missing endpoint")
	}

	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	if baseURL.Scheme != "https" && baseURL.Scheme != "http" {
		return nil, fmt.Errorf("endpoint must use http or https, got %q", baseURL.Scheme)
	}

	if baseURL.Host == "" {
		return nil, errors.New("endpoint must include a host")
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (client *Client) ListInstanceTypes(ctx context.Context) (_ []InstanceType, returnErr error) {
	requestURL := client.baseURL.JoinPath("v1", "instance_types")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create instance types request: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.apiKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list instance types: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close instance types response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, responsePreviewByteLimit))
		if readErr != nil {
			return nil, fmt.Errorf("list instance types: status %s; read error body: %w", response.Status, readErr)
		}

		return nil, fmt.Errorf("list instance types: status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var payload instanceTypesResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode instance types response: %w", err)
	}

	if payload.InstanceTypes == nil {
		return []InstanceType{}, nil
	}

	return payload.InstanceTypes, nil
}
