package baseten

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type DeploymentArchivePayload struct {
	Config                  json.RawMessage   `json:"config"`
	RawConfig               *string           `json:"raw_config,omitempty"`
	EnvironmentName         *string           `json:"environment_name,omitempty"`
	PreserveEnvInstanceType *bool             `json:"preserve_env_instance_type,omitempty"`
	DeployTimeoutMinutes    *int64            `json:"deploy_timeout_minutes,omitempty"`
	DeploymentName          *string           `json:"deployment_name,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
}

type PrepareModelUploadRequest struct {
	Deployment    DeploymentArchivePayload `json:"deployment"`
	Name          *string                  `json:"name,omitempty"`
	TeamID        *string                  `json:"team_id,omitempty"`
	ModelID       *string                  `json:"model_id,omitempty"`
	DryRun        *bool                    `json:"dry_run,omitempty"`
	IsDevelopment *bool                    `json:"is_development,omitempty"`
}

type PrepareModelUploadResponse struct {
	Credentials *AWSCredentials `json:"creds"`
	S3Bucket    *string         `json:"s3_bucket"`
	S3Key       *string         `json:"s3_key"`
	S3Region    *string         `json:"s3_region"`
}

type AWSCredentials struct {
	AccessKeyID     string `json:"aws_access_key_id"`
	SecretAccessKey string `json:"aws_secret_access_key"`
	SessionToken    string `json:"aws_session_token"`
}

type Model struct {
	ID                      string  `json:"id"`
	CreatedAt               string  `json:"created_at"`
	Name                    string  `json:"name"`
	DeploymentsCount        int64   `json:"deployments_count"`
	ProductionDeploymentID  *string `json:"production_deployment_id"`
	DevelopmentDeploymentID *string `json:"development_deployment_id"`
	InstanceTypeName        string  `json:"instance_type_name"`
	TeamName                string  `json:"team_name"`
}

type Deployment struct {
	ID                  string               `json:"id"`
	CreatedAt           string               `json:"created_at"`
	Name                string               `json:"name"`
	ModelID             string               `json:"model_id"`
	IsProduction        bool                 `json:"is_production"`
	IsDevelopment       bool                 `json:"is_development"`
	Status              string               `json:"status"`
	ActiveReplicaCount  int64                `json:"active_replica_count"`
	AutoscalingSettings *AutoscalingSettings `json:"autoscaling_settings"`
	InstanceTypeName    *string              `json:"instance_type_name"`
	Environment         *string              `json:"environment"`
	Labels              map[string]string    `json:"labels"`
}

type CreatedModelDeployment struct {
	Model      Model      `json:"model"`
	Deployment Deployment `json:"deployment"`
}

type CreateModelFromArchiveRequest struct {
	Name                   string
	Deployment             DeploymentArchivePayload
	S3Key                  string
	DisableArchiveDownload bool
	IsDevelopment          bool
}

type modelTombstone struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type deploymentTombstone struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	ModelID string `json:"model_id"`
}

type StatusError struct {
	Operation  string
	Status     string
	StatusCode int
	Body       string
}

func (err StatusError) Error() string {
	return fmt.Sprintf("%s: status %s: %s", err.Operation, err.Status, err.Body)
}

type createModelRequestBody struct {
	Source modelArchiveSource `json:"source"`
}

type modelArchiveSource struct {
	Kind                   string                   `json:"kind"`
	Name                   string                   `json:"name"`
	Deployment             DeploymentArchivePayload `json:"deployment"`
	S3Key                  string                   `json:"s3_key"`
	DisableArchiveDownload bool                     `json:"disable_archive_download"`
	IsDevelopment          bool                     `json:"is_development"`
}

func (client *Client) PrepareModelUpload(ctx context.Context, uploadRequest PrepareModelUploadRequest) (_ PrepareModelUploadResponse, returnErr error) {
	requestBody, err := json.Marshal(uploadRequest)
	if err != nil {
		return PrepareModelUploadResponse{}, fmt.Errorf("encode prepare model upload request: %w", err)
	}

	requestURL := client.baseURL.JoinPath("v1", "prepare_model_upload")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return PrepareModelUploadResponse{}, fmt.Errorf("create prepare model upload request: %w", err)
	}

	client.setJSONHeaders(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return PrepareModelUploadResponse{}, fmt.Errorf("prepare model upload: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close prepare model upload response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return PrepareModelUploadResponse{}, responseStatusError("prepare model upload", response)
	}

	var payload PrepareModelUploadResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return PrepareModelUploadResponse{}, fmt.Errorf("decode prepare model upload response: %w", err)
	}

	return payload, nil
}

func (client *Client) CreateModelFromArchive(ctx context.Context, createRequest CreateModelFromArchiveRequest) (_ CreatedModelDeployment, returnErr error) {
	if createRequest.Name == "" {
		return CreatedModelDeployment{}, errors.New("missing model name")
	}

	if createRequest.S3Key == "" {
		return CreatedModelDeployment{}, errors.New("missing S3 key")
	}

	requestBody, err := json.Marshal(createModelRequestBody{
		Source: modelArchiveSource{
			Kind:                   "model_archive",
			Name:                   createRequest.Name,
			Deployment:             createRequest.Deployment,
			S3Key:                  createRequest.S3Key,
			DisableArchiveDownload: createRequest.DisableArchiveDownload,
			IsDevelopment:          createRequest.IsDevelopment,
		},
	})
	if err != nil {
		return CreatedModelDeployment{}, fmt.Errorf("encode create model request: %w", err)
	}

	requestURL := client.baseURL.JoinPath("v1", "models")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return CreatedModelDeployment{}, fmt.Errorf("create model request: %w", err)
	}

	client.setJSONHeaders(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return CreatedModelDeployment{}, fmt.Errorf("create model: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close create model response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return CreatedModelDeployment{}, responseStatusError("create model", response)
	}

	var payload CreatedModelDeployment
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return CreatedModelDeployment{}, fmt.Errorf("decode create model response: %w", err)
	}

	return payload, nil
}

func (client *Client) GetDeployment(ctx context.Context, modelID string, deploymentID string) (_ Deployment, returnErr error) {
	if modelID == "" {
		return Deployment{}, errors.New("missing model ID")
	}

	if deploymentID == "" {
		return Deployment{}, errors.New("missing deployment ID")
	}

	requestURL := client.baseURL.JoinPath("v1", "models", modelID, "deployments", deploymentID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), http.NoBody)
	if err != nil {
		return Deployment{}, fmt.Errorf("create get deployment request: %w", err)
	}

	client.setJSONHeaders(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return Deployment{}, fmt.Errorf("get deployment: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close get deployment response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return Deployment{}, responseStatusError("get deployment", response)
	}

	var payload Deployment
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Deployment{}, fmt.Errorf("decode get deployment response: %w", err)
	}

	return payload, nil
}

func (client *Client) DeleteDeployment(ctx context.Context, modelID string, deploymentID string) (_ bool, returnErr error) {
	if modelID == "" {
		return false, errors.New("missing model ID")
	}

	if deploymentID == "" {
		return false, errors.New("missing deployment ID")
	}

	requestURL := client.baseURL.JoinPath("v1", "models", modelID, "deployments", deploymentID)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL.String(), http.NoBody)
	if err != nil {
		return false, fmt.Errorf("create delete deployment request: %w", err)
	}

	client.setJSONHeaders(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("delete deployment: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close delete deployment response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return false, responseStatusError("delete deployment", response)
	}

	var payload deploymentTombstone
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode delete deployment response: %w", err)
	}

	return payload.Deleted, nil
}

func (client *Client) DeleteModel(ctx context.Context, modelID string) (_ bool, returnErr error) {
	if modelID == "" {
		return false, errors.New("missing model ID")
	}

	requestURL := client.baseURL.JoinPath("v1", "models", modelID)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL.String(), http.NoBody)
	if err != nil {
		return false, fmt.Errorf("create delete model request: %w", err)
	}

	client.setJSONHeaders(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("delete model: %w", err)
	}
	defer func() {
		closeErr := response.Body.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close delete model response body: %w", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		return false, responseStatusError("delete model", response)
	}

	var payload modelTombstone
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode delete model response: %w", err)
	}

	return payload.Deleted, nil
}

func (client *Client) setJSONHeaders(request *http.Request) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")
}

func responseStatusError(operation string, response *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, responsePreviewByteLimit))
	if readErr != nil {
		return fmt.Errorf("%s: status %s; read error body: %w", operation, response.Status, readErr)
	}

	return StatusError{
		Operation:  operation,
		Status:     response.Status,
		StatusCode: response.StatusCode,
		Body:       strings.TrimSpace(string(body)),
	}
}
