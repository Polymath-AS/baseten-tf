package baseten

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrepareModelUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/prepare_model_upload" {
			t.Fatalf("path = %q, want /v1/prepare_model_upload", request.URL.Path)
		}

		if request.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", request.Method)
		}

		var body PrepareModelUploadRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		if body.Name == nil || *body.Name != "custom-model" {
			t.Fatalf("name = %v, want custom-model", body.Name)
		}

		if string(body.Deployment.Config) != `{"model_name":"demo"}` {
			t.Fatalf("config = %s, want model config", body.Deployment.Config)
		}

		bucket := "baseten-upload"
		s3Key := "archives/model.tar.gz"
		region := "us-west-2"
		responseBody := PrepareModelUploadResponse{
			Credentials: &AWSCredentials{
				AccessKeyID:     "access-key",
				SecretAccessKey: "secret-key",
				SessionToken:    "session-token",
			},
			S3Bucket: &bucket,
			S3Key:    &s3Key,
			S3Region: &region,
		}
		response.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(response).Encode(responseBody); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	name := "custom-model"
	result, err := client.PrepareModelUpload(context.Background(), PrepareModelUploadRequest{
		Name: &name,
		Deployment: DeploymentArchivePayload{
			Config: json.RawMessage(`{"model_name":"demo"}`),
		},
	})
	if err != nil {
		t.Fatalf("PrepareModelUpload: %v", err)
	}

	if result.S3Key == nil || *result.S3Key != "archives/model.tar.gz" {
		t.Fatalf("S3Key = %v, want archives/model.tar.gz", result.S3Key)
	}
}

func TestCreateModelFromArchive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", request.URL.Path)
		}

		if request.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", request.Method)
		}

		var body createModelRequestBody
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		if body.Source.Kind != "model_archive" {
			t.Fatalf("kind = %q, want model_archive", body.Source.Kind)
		}

		if body.Source.Name != "custom-model" {
			t.Fatalf("name = %q, want custom-model", body.Source.Name)
		}

		if body.Source.S3Key != "archives/model.tar.gz" {
			t.Fatalf("s3_key = %q, want archives/model.tar.gz", body.Source.S3Key)
		}

		response.Header().Set("Content-Type", "application/json")
		responseBody := CreatedModelDeployment{
			Model: Model{
				ID:               "model-123",
				CreatedAt:        "2026-06-23T00:00:00Z",
				Name:             "custom-model",
				DeploymentsCount: 1,
				InstanceTypeName: "A10G",
				TeamName:         "default",
			},
			Deployment: Deployment{
				ID:                 "deployment-456",
				CreatedAt:          "2026-06-23T00:00:00Z",
				Name:               "custom-model",
				ModelID:            "model-123",
				Status:             "BUILDING",
				ActiveReplicaCount: 0,
			},
		}
		if err := json.NewEncoder(response).Encode(responseBody); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.CreateModelFromArchive(context.Background(), CreateModelFromArchiveRequest{
		Name:  "custom-model",
		S3Key: "archives/model.tar.gz",
		Deployment: DeploymentArchivePayload{
			Config: json.RawMessage(`{"model_name":"demo"}`),
		},
	})
	if err != nil {
		t.Fatalf("CreateModelFromArchive: %v", err)
	}

	if result.Model.ID != "model-123" {
		t.Fatalf("model ID = %q, want model-123", result.Model.ID)
	}

	if result.Deployment.ID != "deployment-456" {
		t.Fatalf("deployment ID = %q, want deployment-456", result.Deployment.ID)
	}
}

func TestGetDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models/model-123/deployments/deployment-456" {
			t.Fatalf("path = %q, want deployment path", request.URL.Path)
		}

		response.Header().Set("Content-Type", "application/json")
		responseBody := Deployment{
			ID:                 "deployment-456",
			CreatedAt:          "2026-06-23T00:00:00Z",
			Name:               "custom-model",
			ModelID:            "model-123",
			Status:             "ACTIVE",
			ActiveReplicaCount: 0,
		}
		if err := json.NewEncoder(response).Encode(responseBody); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	deployment, err := client.GetDeployment(context.Background(), "model-123", "deployment-456")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}

	if deployment.Status != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE", deployment.Status)
	}
}

func TestDeleteDeploymentAndModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")

		switch request.URL.Path {
		case "/v1/models/model-123/deployments/deployment-456":
			if request.Method != http.MethodDelete {
				t.Fatalf("method = %q, want DELETE", request.Method)
			}

			body := deploymentTombstone{ID: "deployment-456", Deleted: true, ModelID: "model-123"}
			if err := json.NewEncoder(response).Encode(body); err != nil {
				t.Fatalf("encode deployment tombstone: %v", err)
			}

		case "/v1/models/model-123":
			if request.Method != http.MethodDelete {
				t.Fatalf("method = %q, want DELETE", request.Method)
			}

			body := modelTombstone{ID: "model-123", Deleted: true}
			if err := json.NewEncoder(response).Encode(body); err != nil {
				t.Fatalf("encode model tombstone: %v", err)
			}

		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	deploymentDeleted, err := client.DeleteDeployment(context.Background(), "model-123", "deployment-456")
	if err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}

	if !deploymentDeleted {
		t.Fatal("DeleteDeployment returned false")
	}

	modelDeleted, err := client.DeleteModel(context.Background(), "model-123")
	if err != nil {
		t.Fatalf("DeleteModel: %v", err)
	}

	if !modelDeleted {
		t.Fatal("DeleteModel returned false")
	}
}
