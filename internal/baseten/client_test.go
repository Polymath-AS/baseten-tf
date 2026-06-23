package baseten

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListInstanceTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/instance_types" {
			t.Fatalf("path = %q, want /v1/instance_types", request.URL.Path)
		}

		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want Bearer test-key", request.Header.Get("Authorization"))
		}

		response.Header().Set("Content-Type", "application/json")
		gpuType := "A10G"
		gpuMemoryLimitMiB := int64(24576)
		body := instanceTypesResponse{
			InstanceTypes: []InstanceType{
				{
					ID:                "A10G",
					Name:              "A10G",
					MemoryLimitMiB:    32768,
					MillicpuLimit:     4000,
					GPUCount:          1,
					GPUType:           &gpuType,
					GPUMemoryLimitMiB: &gpuMemoryLimitMiB,
				},
			},
		}
		if err := json.NewEncoder(response).Encode(body); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	instanceTypes, err := client.ListInstanceTypes(context.Background())
	if err != nil {
		t.Fatalf("ListInstanceTypes: %v", err)
	}

	if len(instanceTypes) != 1 {
		t.Fatalf("len(instanceTypes) = %d, want 1", len(instanceTypes))
	}

	instanceType := instanceTypes[0]
	if instanceType.ID != "A10G" {
		t.Fatalf("ID = %q, want A10G", instanceType.ID)
	}

	if instanceType.GPUType == nil || *instanceType.GPUType != "A10G" {
		t.Fatalf("GPUType = %v, want A10G", instanceType.GPUType)
	}
}

func TestNewClientRejectsInvalidEndpoint(t *testing.T) {
	_, err := NewClient("test-key", "localhost:8080")
	if err == nil {
		t.Fatal("NewClient accepted an endpoint without a URL scheme")
	}
}

func TestUpdateDeploymentAutoscalingSettingsAllowsScaleToZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models/model-123/deployments/deployment-456/autoscaling_settings" {
			t.Fatalf("path = %q, want autoscaling settings path", request.URL.Path)
		}

		if request.Method != http.MethodPatch {
			t.Fatalf("method = %q, want PATCH", request.Method)
		}

		var body struct {
			MinReplica        *int64 `json:"min_replica"`
			MaxReplica        *int64 `json:"max_replica"`
			ScaleDownDelay    *int64 `json:"scale_down_delay"`
			ConcurrencyTarget *int64 `json:"concurrency_target"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		if body.MinReplica == nil || *body.MinReplica != 0 {
			t.Fatalf("min_replica = %v, want 0", body.MinReplica)
		}

		if body.MaxReplica == nil || *body.MaxReplica != 1 {
			t.Fatalf("max_replica = %v, want 1", body.MaxReplica)
		}

		response.Header().Set("Content-Type", "application/json")
		bodyResponse := UpdateAutoscalingSettingsResponse{Status: "ACCEPTED", Message: "queued"}
		if err := json.NewEncoder(response).Encode(bodyResponse); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	minReplica := int64(0)
	maxReplica := int64(1)
	scaleDownDelay := int64(120)
	concurrencyTarget := int64(2)
	result, err := client.UpdateDeploymentAutoscalingSettings(
		context.Background(),
		"model-123",
		"deployment-456",
		AutoscalingSettings{
			MinReplica:        &minReplica,
			MaxReplica:        &maxReplica,
			ScaleDownDelay:    &scaleDownDelay,
			ConcurrencyTarget: &concurrencyTarget,
		},
	)
	if err != nil {
		t.Fatalf("UpdateDeploymentAutoscalingSettings: %v", err)
	}

	if result.Status != "ACCEPTED" {
		t.Fatalf("Status = %q, want ACCEPTED", result.Status)
	}
}

func TestUpdateDeploymentAutoscalingSettingsRejectsMissingIDs(t *testing.T) {
	client, err := NewClient("test-key", "https://api.baseten.co")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.UpdateDeploymentAutoscalingSettings(context.Background(), "", "deployment-456", AutoscalingSettings{})
	if err == nil {
		t.Fatal("UpdateDeploymentAutoscalingSettings accepted a missing model ID")
	}

	_, err = client.UpdateDeploymentAutoscalingSettings(context.Background(), "model-123", "", AutoscalingSettings{})
	if err == nil {
		t.Fatal("UpdateDeploymentAutoscalingSettings accepted a missing deployment ID")
	}
}
