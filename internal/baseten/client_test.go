package baseten

import (
	"context"
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
		_, err := response.Write([]byte(`{
			"instance_types": [
				{
					"id": "A10G",
					"name": "A10G",
					"memory_limit_mib": 32768,
					"millicpu_limit": 4000,
					"gpu_count": 1,
					"gpu_type": "A10G",
					"gpu_memory_limit_mib": 24576
				}
			]
		}`))
		if err != nil {
			t.Fatalf("write response: %v", err)
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
