package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polymath-as/baseten-tf/internal/baseten"
)

type instanceTypesDataSource struct {
	client *baseten.Client
}

type instanceTypesDataSourceModel struct {
	ID            types.String        `tfsdk:"id"`
	InstanceTypes []instanceTypeModel `tfsdk:"instance_types"`
}

type instanceTypeModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	MemoryLimitMiB    types.Int64  `tfsdk:"memory_limit_mib"`
	MillicpuLimit     types.Int64  `tfsdk:"millicpu_limit"`
	GPUCount          types.Int64  `tfsdk:"gpu_count"`
	GPUType           types.String `tfsdk:"gpu_type"`
	GPUMemoryLimitMiB types.Int64  `tfsdk:"gpu_memory_limit_mib"`
}

func NewInstanceTypesDataSource() datasource.DataSource {
	return &instanceTypesDataSource{}
}

func (dataSource *instanceTypesDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_instance_types"
}

func (dataSource *instanceTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Lists Baseten instance types available to the authenticated account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable Terraform data source identifier.",
			},
			"instance_types": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Baseten instance types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Baseten instance type identifier.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Baseten instance type display name.",
						},
						"memory_limit_mib": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Memory limit in mebibytes.",
						},
						"millicpu_limit": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "CPU limit in millicpu.",
						},
						"gpu_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Number of GPUs.",
						},
						"gpu_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "GPU type, when the instance includes a GPU.",
						},
						"gpu_memory_limit_mib": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "GPU memory limit in mebibytes, when present.",
						},
					},
				},
			},
		},
	}
}

func (dataSource *instanceTypesDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	client, ok := request.ProviderData.(*baseten.Client)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected provider data",
			"The Baseten provider configured data that the instance types data source cannot use.",
		)
		return
	}

	dataSource.client = client
}

func (dataSource *instanceTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, response *datasource.ReadResponse) {
	instanceTypes, err := dataSource.client.ListInstanceTypes(ctx)
	if err != nil {
		response.Diagnostics.AddError(
			"Unable to list Baseten instance types",
			err.Error(),
		)
		return
	}

	state := instanceTypesDataSourceModel{
		ID:            types.StringValue("instance_types"),
		InstanceTypes: make([]instanceTypeModel, 0, len(instanceTypes)),
	}

	for _, instanceType := range instanceTypes {
		state.InstanceTypes = append(state.InstanceTypes, instanceTypeModel{
			ID:                types.StringValue(instanceType.ID),
			Name:              types.StringValue(instanceType.Name),
			MemoryLimitMiB:    types.Int64Value(instanceType.MemoryLimitMiB),
			MillicpuLimit:     types.Int64Value(instanceType.MillicpuLimit),
			GPUCount:          types.Int64Value(instanceType.GPUCount),
			GPUType:           stringPointerValue(instanceType.GPUType),
			GPUMemoryLimitMiB: int64PointerValue(instanceType.GPUMemoryLimitMiB),
		})
	}

	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func stringPointerValue(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}

	return types.StringValue(*value)
}

func int64PointerValue(value *int64) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}

	return types.Int64Value(*value)
}
