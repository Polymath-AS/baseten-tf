package provider

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polymath-as/baseten-tf/internal/baseten"
)

type fakeCustomModelClient struct {
	preparedName            string
	createdName             string
	createdS3Key            string
	preparedEnvironmentName *string
	autoscalingModelID      string
	autoscalingDeploymentID string
	autoscalingMin          *int64
}

func (client *fakeCustomModelClient) PrepareModelUpload(_ context.Context, request baseten.PrepareModelUploadRequest) (baseten.PrepareModelUploadResponse, error) {
	if request.Name != nil {
		client.preparedName = *request.Name
	}
	client.preparedEnvironmentName = request.Deployment.EnvironmentName

	bucket := "bucket"
	key := "archive.tar.gz"
	region := "us-west-2"
	return baseten.PrepareModelUploadResponse{
		Credentials: &baseten.AWSCredentials{AccessKeyID: "access", SecretAccessKey: "secret", SessionToken: "session"},
		S3Bucket:    &bucket,
		S3Key:       &key,
		S3Region:    &region,
	}, nil
}

func (client *fakeCustomModelClient) CreateModelFromArchive(_ context.Context, request baseten.CreateModelFromArchiveRequest) (baseten.CreatedModelDeployment, error) {
	client.createdName = request.Name
	client.createdS3Key = request.S3Key

	return baseten.CreatedModelDeployment{
		Model:      baseten.Model{ID: "model-123"},
		Deployment: baseten.Deployment{ID: "deployment-456", ModelID: "model-123", Status: "BUILDING"},
	}, nil
}

func (client *fakeCustomModelClient) UpdateDeploymentAutoscalingSettings(_ context.Context, modelID string, deploymentID string, settings baseten.AutoscalingSettings) (baseten.UpdateAutoscalingSettingsResponse, error) {
	client.autoscalingModelID = modelID
	client.autoscalingDeploymentID = deploymentID
	client.autoscalingMin = settings.MinReplica
	return baseten.UpdateAutoscalingSettingsResponse{Status: "ACCEPTED", Message: "queued"}, nil
}

func (client *fakeCustomModelClient) GetDeployment(context.Context, string, string) (baseten.Deployment, error) {
	return baseten.Deployment{ID: "deployment-456", ModelID: "model-123", Status: "ACTIVE", ActiveReplicaCount: 0}, nil
}

func (client *fakeCustomModelClient) DeleteDeployment(context.Context, string, string) (bool, error) {
	return true, nil
}

func (client *fakeCustomModelClient) DeleteModel(context.Context, string) (bool, error) {
	return true, nil
}

func TestCreateCustomModelOrchestratesUploadCreateAndScaleToZero(t *testing.T) {
	client := &fakeCustomModelClient{}
	archiveBytes := []byte("archive")
	uploadedBytes := ""

	writeArchive := func(sourcePath string, writer io.Writer) error {
		if sourcePath != "/models/demo" {
			t.Fatalf("sourcePath = %q, want /models/demo", sourcePath)
		}

		_, err := writer.Write(archiveBytes)
		return err
	}

	uploadArchive := func(_ context.Context, _ baseten.PrepareModelUploadResponse, reader io.Reader) error {
		body, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		uploadedBytes = string(body)
		return nil
	}

	output, err := createCustomModel(context.Background(), client, customModelInput{
		Name:            "custom-model",
		SourcePath:      "/models/demo",
		ConfigJSON:      `{"model_name":"demo"}`,
		EnvironmentName: stringValuePointer("production"),
		MinReplica:      0,
		MaxReplica:      1,
	}, writeArchive, uploadArchive)
	if err != nil {
		t.Fatalf("createCustomModel: %v", err)
	}

	if client.preparedName != "custom-model" {
		t.Fatalf("preparedName = %q, want custom-model", client.preparedName)
	}

	if client.createdS3Key != "archive.tar.gz" {
		t.Fatalf("createdS3Key = %q, want archive.tar.gz", client.createdS3Key)
	}

	if client.preparedEnvironmentName == nil || *client.preparedEnvironmentName != "production" {
		t.Fatalf("preparedEnvironmentName = %v, want production", client.preparedEnvironmentName)
	}

	if uploadedBytes != "archive" {
		t.Fatalf("uploadedBytes = %q, want archive", uploadedBytes)
	}

	if client.autoscalingMin == nil || *client.autoscalingMin != 0 {
		t.Fatalf("autoscaling min = %v, want 0", client.autoscalingMin)
	}

	if output.ModelID != "model-123" || output.DeploymentStatus != "ACTIVE" {
		t.Fatalf("output = %#v, want created active model", output)
	}
}

func TestCreateCustomModelRejectsInvalidConfigJSON(t *testing.T) {
	client := &fakeCustomModelClient{}
	writeArchive := func(_ string, writer io.Writer) error {
		_, err := writer.Write([]byte("archive"))
		return err
	}

	uploadArchive := func(context.Context, baseten.PrepareModelUploadResponse, io.Reader) error {
		return nil
	}

	_, err := createCustomModel(context.Background(), client, customModelInput{
		Name:       "custom-model",
		SourcePath: "/models/demo",
		ConfigJSON: `{invalid`,
		MinReplica: 0,
		MaxReplica: 1,
	}, writeArchive, uploadArchive)
	if err == nil {
		t.Fatal("createCustomModel accepted invalid config JSON")
	}
}

func TestUpdateCustomModelAutoscalingUsesStateIDs(t *testing.T) {
	client := &fakeCustomModelClient{}
	state := customModelResourceModel{
		ModelID:      types.StringValue("model-from-state"),
		DeploymentID: types.StringValue("deployment-from-state"),
	}
	plan := customModelResourceModel{
		ModelID:      types.StringUnknown(),
		DeploymentID: types.StringUnknown(),
		MinReplica:   types.Int64Value(0),
		MaxReplica:   types.Int64Value(2),
	}

	output, err := updateCustomModelAutoscaling(context.Background(), client, state, plan)
	if err != nil {
		t.Fatalf("updateCustomModelAutoscaling: %v", err)
	}

	if client.autoscalingModelID != "model-from-state" {
		t.Fatalf("autoscalingModelID = %q, want model-from-state", client.autoscalingModelID)
	}

	if client.autoscalingDeploymentID != "deployment-from-state" {
		t.Fatalf("autoscalingDeploymentID = %q, want deployment-from-state", client.autoscalingDeploymentID)
	}

	if output.DeploymentStatus != "ACTIVE" {
		t.Fatalf("DeploymentStatus = %q, want ACTIVE", output.DeploymentStatus)
	}
}

func TestOptionalHelpers(t *testing.T) {
	value := int64ValuePointer(0)
	if value == nil || *value != 0 {
		t.Fatalf("int64ValuePointer(0) = %v, want pointer to zero", value)
	}

	var buffer bytes.Buffer
	if buffer.Len() != 0 {
		t.Fatalf("buffer length = %d, want 0", buffer.Len())
	}
}

func stringValuePointer(value string) *string {
	return &value
}
