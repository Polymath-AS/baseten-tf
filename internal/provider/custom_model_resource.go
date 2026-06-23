package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	schemavalidator "github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polymath-as/baseten-tf/internal/archive"
	"github.com/polymath-as/baseten-tf/internal/baseten"
)

type customModelClient interface {
	PrepareModelUpload(context.Context, baseten.PrepareModelUploadRequest) (baseten.PrepareModelUploadResponse, error)
	CreateModelFromArchive(context.Context, baseten.CreateModelFromArchiveRequest) (baseten.CreatedModelDeployment, error)
	UpdateDeploymentAutoscalingSettings(context.Context, string, string, baseten.AutoscalingSettings) (baseten.UpdateAutoscalingSettingsResponse, error)
	GetDeployment(context.Context, string, string) (baseten.Deployment, error)
	DeleteDeployment(context.Context, string, string) (bool, error)
	DeleteModel(context.Context, string) (bool, error)
}

type customModelResource struct {
	client customModelClient
}

type customModelResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	SourcePath              types.String `tfsdk:"source_path"`
	SourceHash              types.String `tfsdk:"source_hash"`
	ConfigJSON              types.String `tfsdk:"config_json"`
	RawConfig               types.String `tfsdk:"raw_config"`
	EnvironmentName         types.String `tfsdk:"environment_name"`
	PreserveEnvInstanceType types.Bool   `tfsdk:"preserve_env_instance_type"`
	DeploymentName          types.String `tfsdk:"deployment_name"`
	MinReplica              types.Int64  `tfsdk:"min_replica"`
	MaxReplica              types.Int64  `tfsdk:"max_replica"`
	ScaleDownDelay          types.Int64  `tfsdk:"scale_down_delay"`
	ConcurrencyTarget       types.Int64  `tfsdk:"concurrency_target"`
	ModelID                 types.String `tfsdk:"model_id"`
	DeploymentID            types.String `tfsdk:"deployment_id"`
	DeploymentStatus        types.String `tfsdk:"deployment_status"`
	ActiveReplicaCount      types.Int64  `tfsdk:"active_replica_count"`
	DisableArchiveAccess    types.Bool   `tfsdk:"disable_archive_access"`
}

type customModelInput struct {
	Name                    string
	SourcePath              string
	ConfigJSON              string
	RawConfig               *string
	EnvironmentName         *string
	PreserveEnvInstanceType *bool
	DeploymentName          *string
	MinReplica              int64
	MaxReplica              int64
	ScaleDownDelay          *int64
	ConcurrencyTarget       *int64
	DisableArchiveAccess    bool
}

type customModelOutput struct {
	ModelID            string
	DeploymentID       string
	DeploymentStatus   string
	ActiveReplicaCount int64
}

type archiveWriter func(string, io.Writer) error

type archiveUploader func(context.Context, baseten.PrepareModelUploadResponse, io.Reader) error

var writeCustomModelArchive archiveWriter = archive.WriteDirectoryTarGzip
var uploadCustomModelArchive archiveUploader = baseten.UploadModelArchive

const deploymentPollInterval = 30 * time.Second

func NewCustomModelResource() resource.Resource {
	return &customModelResource{}
}

func (customResource *customModelResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_custom_model"
}

func (customResource *customModelResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Deploys a Baseten custom model archive and configures deployment autoscaling.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Path to the local model directory to archive and upload.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_hash": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Caller-provided hash of the local model source. Changing this value replaces the deployment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config_json": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Baseten deployment config as a JSON object string.",
				Validators: []schemavalidator.String{
					jsonObjectStringValidator{},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"raw_config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Original config.yaml contents to persist with the deployment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Baseten environment to push to, such as production, for a stable endpoint.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"preserve_env_instance_type": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When environment_name targets an existing environment, preserve that environment instance type instead of the config instance type.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"deployment_name": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"min_replica": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Minimum replica count. Use 0 to allow scale-to-zero.",
				Validators: []schemavalidator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"max_replica": schema.Int64Attribute{
				Required: true,
				Validators: []schemavalidator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"scale_down_delay": schema.Int64Attribute{
				Optional: true,
				Validators: []schemavalidator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"concurrency_target": schema.Int64Attribute{
				Optional: true,
				Validators: []schemavalidator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"model_id":             schema.StringAttribute{Computed: true},
			"deployment_id":        schema.StringAttribute{Computed: true},
			"deployment_status":    schema.StringAttribute{Computed: true},
			"active_replica_count": schema.Int64Attribute{Computed: true},
			"disable_archive_access": schema.BoolAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (customResource *customModelResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	client, ok := request.ProviderData.(customModelClient)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected provider data",
			"The Baseten provider configured data that the custom model resource cannot use.",
		)
		return
	}

	customResource.client = client
}

func (customResource *customModelResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan customModelResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	input := customModelInput{
		Name:                    plan.Name.ValueString(),
		SourcePath:              plan.SourcePath.ValueString(),
		ConfigJSON:              plan.ConfigJSON.ValueString(),
		RawConfig:               optionalStringPointer(plan.RawConfig),
		EnvironmentName:         optionalStringPointer(plan.EnvironmentName),
		PreserveEnvInstanceType: optionalBoolPointer(plan.PreserveEnvInstanceType),
		DeploymentName:          optionalStringPointer(plan.DeploymentName),
		MinReplica:              plan.MinReplica.ValueInt64(),
		MaxReplica:              plan.MaxReplica.ValueInt64(),
		ScaleDownDelay:          optionalInt64Pointer(plan.ScaleDownDelay),
		ConcurrencyTarget:       optionalInt64Pointer(plan.ConcurrencyTarget),
		DisableArchiveAccess:    optionalBoolValue(plan.DisableArchiveAccess),
	}

	output, err := createCustomModel(ctx, customResource.client, input, writeCustomModelArchive, uploadCustomModelArchive)
	if err != nil {
		response.Diagnostics.AddError("Unable to create Baseten custom model", err.Error())
		return
	}

	plan.ID = types.StringValue(output.ModelID + ":" + output.DeploymentID)
	plan.ModelID = types.StringValue(output.ModelID)
	plan.DeploymentID = types.StringValue(output.DeploymentID)
	plan.DeploymentStatus = types.StringValue(output.DeploymentStatus)
	plan.ActiveReplicaCount = types.Int64Value(output.ActiveReplicaCount)

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (customResource *customModelResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state customModelResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	deployment, err := customResource.client.GetDeployment(ctx, state.ModelID.ValueString(), state.DeploymentID.ValueString())
	if err != nil {
		var statusError baseten.StatusError
		if errors.As(err, &statusError) && statusError.StatusCode == 404 {
			response.State.RemoveResource(ctx)
			return
		}

		response.Diagnostics.AddError("Unable to read Baseten custom model", err.Error())
		return
	}

	state.DeploymentStatus = types.StringValue(deployment.Status)
	state.ActiveReplicaCount = types.Int64Value(deployment.ActiveReplicaCount)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (customResource *customModelResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan customModelResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	var state customModelResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	output, err := updateCustomModelAutoscaling(ctx, customResource.client, state, plan)
	if err != nil {
		response.Diagnostics.AddError("Unable to update Baseten custom model autoscaling", err.Error())
		return
	}

	plan.ID = state.ID
	plan.ModelID = state.ModelID
	plan.DeploymentID = state.DeploymentID
	plan.DeploymentStatus = types.StringValue(output.DeploymentStatus)
	plan.ActiveReplicaCount = types.Int64Value(output.ActiveReplicaCount)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func updateCustomModelAutoscaling(ctx context.Context, client customModelClient, state customModelResourceModel, plan customModelResourceModel) (customModelOutput, error) {
	settings := baseten.AutoscalingSettings{
		MinReplica:        int64ValuePointer(plan.MinReplica.ValueInt64()),
		MaxReplica:        int64ValuePointer(plan.MaxReplica.ValueInt64()),
		ScaleDownDelay:    optionalInt64Pointer(plan.ScaleDownDelay),
		ConcurrencyTarget: optionalInt64Pointer(plan.ConcurrencyTarget),
	}

	modelID := state.ModelID.ValueString()
	deploymentID := state.DeploymentID.ValueString()
	_, err := client.UpdateDeploymentAutoscalingSettings(ctx, modelID, deploymentID, settings)
	if err != nil {
		return customModelOutput{}, err
	}

	deployment, err := client.GetDeployment(ctx, modelID, deploymentID)
	if err != nil {
		return customModelOutput{}, err
	}

	return customModelOutput{
		ModelID:            modelID,
		DeploymentID:       deploymentID,
		DeploymentStatus:   deployment.Status,
		ActiveReplicaCount: deployment.ActiveReplicaCount,
	}, nil
}

func (customResource *customModelResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state customModelResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	if err := deleteCustomModel(ctx, customResource.client, state.ModelID.ValueString(), state.DeploymentID.ValueString()); err != nil {
		response.Diagnostics.AddError("Unable to delete Baseten custom model", err.Error())
		return
	}
}

func deleteCustomModel(ctx context.Context, client customModelClient, modelID string, deploymentID string) error {
	_, deploymentErr := client.DeleteDeployment(ctx, modelID, deploymentID)
	if deploymentErr != nil && !isStatusCode(deploymentErr, 404) {
		return fmt.Errorf("delete deployment: %w", deploymentErr)
	}

	_, modelErr := client.DeleteModel(ctx, modelID)
	if modelErr != nil && !isStatusCode(modelErr, 404) {
		return fmt.Errorf("delete model: %w", modelErr)
	}

	return nil
}

func isStatusCode(err error, statusCode int) bool {
	var statusError baseten.StatusError
	return errors.As(err, &statusError) && statusError.StatusCode == statusCode
}

func createCustomModel(ctx context.Context, client customModelClient, input customModelInput, writeArchive archiveWriter, uploadArchive archiveUploader) (customModelOutput, error) {
	if !json.Valid([]byte(input.ConfigJSON)) {
		return customModelOutput{}, errors.New("config_json must be valid JSON")
	}

	deploymentPayload := baseten.DeploymentArchivePayload{
		Config:                  json.RawMessage(input.ConfigJSON),
		RawConfig:               input.RawConfig,
		EnvironmentName:         input.EnvironmentName,
		PreserveEnvInstanceType: input.PreserveEnvInstanceType,
		DeploymentName:          input.DeploymentName,
		DeployTimeoutMinutes:    nil,
	}

	uploadRequestName := input.Name
	uploadResponse, err := client.PrepareModelUpload(ctx, baseten.PrepareModelUploadRequest{
		Name:       &uploadRequestName,
		Deployment: deploymentPayload,
	})
	if err != nil {
		return customModelOutput{}, fmt.Errorf("prepare model upload: %w", err)
	}

	if err := streamArchiveUpload(ctx, input.SourcePath, uploadResponse, writeArchive, uploadArchive); err != nil {
		return customModelOutput{}, fmt.Errorf("upload model archive: %w", err)
	}

	if uploadResponse.S3Key == nil || *uploadResponse.S3Key == "" {
		return customModelOutput{}, errors.New("prepare model upload returned no S3 key")
	}

	created, err := client.CreateModelFromArchive(ctx, baseten.CreateModelFromArchiveRequest{
		Name:                   input.Name,
		Deployment:             deploymentPayload,
		S3Key:                  *uploadResponse.S3Key,
		DisableArchiveDownload: input.DisableArchiveAccess,
	})
	if err != nil {
		return customModelOutput{}, fmt.Errorf("create model from archive: %w", err)
	}

	if _, err := waitForDeploymentReady(ctx, client, created.Model.ID, created.Deployment.ID, deploymentPollInterval); err != nil {
		return customModelOutput{}, fmt.Errorf("wait for deployment readiness: %w", err)
	}

	_, err = client.UpdateDeploymentAutoscalingSettings(ctx, created.Model.ID, created.Deployment.ID, baseten.AutoscalingSettings{
		MinReplica:        int64ValuePointer(input.MinReplica),
		MaxReplica:        int64ValuePointer(input.MaxReplica),
		ScaleDownDelay:    input.ScaleDownDelay,
		ConcurrencyTarget: input.ConcurrencyTarget,
	})
	if err != nil {
		return customModelOutput{}, fmt.Errorf("update autoscaling settings: %w", err)
	}

	deployment, err := client.GetDeployment(ctx, created.Model.ID, created.Deployment.ID)
	if err != nil {
		return customModelOutput{}, fmt.Errorf("read created deployment: %w", err)
	}

	return customModelOutput{
		ModelID:            created.Model.ID,
		DeploymentID:       created.Deployment.ID,
		DeploymentStatus:   deployment.Status,
		ActiveReplicaCount: deployment.ActiveReplicaCount,
	}, nil
}

func streamArchiveUpload(ctx context.Context, sourcePath string, uploadResponse baseten.PrepareModelUploadResponse, writeArchive archiveWriter, uploadArchive archiveUploader) error {
	reader, writer := io.Pipe()
	archiveErr := make(chan error, 1)

	go func() {
		writeErr := writeArchive(sourcePath, writer)
		if writeErr != nil {
			archiveErr <- writeErr
			_ = writer.CloseWithError(writeErr)
			return
		}

		archiveErr <- writer.Close()
	}()

	uploadErr := uploadArchive(ctx, uploadResponse, reader)
	if uploadErr != nil {
		_ = reader.CloseWithError(uploadErr)
	}

	writeErr := <-archiveErr
	if uploadErr != nil {
		return uploadErr
	}

	if writeErr != nil {
		return fmt.Errorf("write archive stream: %w", writeErr)
	}

	return nil
}

func waitForDeploymentReady(ctx context.Context, client customModelClient, modelID string, deploymentID string, pollInterval time.Duration) (baseten.Deployment, error) {
	for {
		deployment, err := client.GetDeployment(ctx, modelID, deploymentID)
		if err != nil {
			return baseten.Deployment{}, err
		}

		switch deployment.Status {
		case "ACTIVE", "SCALED_TO_ZERO":
			return deployment, nil
		case "DEPLOY_FAILED", "BUILD_FAILED", "BUILD_STOPPED", "FAILED", "UNHEALTHY":
			return baseten.Deployment{}, fmt.Errorf("deployment %s reached terminal status %s", deploymentID, deployment.Status)
		}

		select {
		case <-ctx.Done():
			return baseten.Deployment{}, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func optionalStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	stringValue := value.ValueString()
	return &stringValue
}

func optionalInt64Pointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	return int64ValuePointer(value.ValueInt64())
}

func int64ValuePointer(value int64) *int64 {
	return &value
}

func optionalBoolValue(value types.Bool) bool {
	if value.IsNull() || value.IsUnknown() {
		return false
	}

	return value.ValueBool()
}

func optionalBoolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	boolValue := value.ValueBool()
	return &boolValue
}
