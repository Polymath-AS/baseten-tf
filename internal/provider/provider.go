package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polymath-as/baseten-tf/internal/baseten"
)

const defaultEndpoint = "https://api.baseten.co"

type basetenProvider struct {
	version string
}

type providerConfigModel struct {
	APIKey   types.String `tfsdk:"api_key"`
	Endpoint types.String `tfsdk:"endpoint"`
}

func New(version string) func() frameworkprovider.Provider {
	return func() frameworkprovider.Provider {
		return &basetenProvider{version: version}
	}
}

func (provider *basetenProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "baseten"
	response.Version = provider.version
}

func (provider *basetenProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for Baseten.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Baseten API key. Can also be set with `BASETEN_API_KEY`.",
				Optional:            true,
				Sensitive:           true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Baseten Management API endpoint.",
				Optional:            true,
			},
		},
	}
}

func (provider *basetenProvider) Configure(ctx context.Context, request frameworkprovider.ConfigureRequest, response *frameworkprovider.ConfigureResponse) {
	var config providerConfigModel

	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("BASETEN_API_KEY")
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}

	if apiKey == "" {
		response.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Baseten API key",
			"Set api_key in the provider configuration or set BASETEN_API_KEY.",
		)
		return
	}

	endpoint := defaultEndpoint
	if !config.Endpoint.IsNull() && !config.Endpoint.IsUnknown() {
		endpoint = config.Endpoint.ValueString()
	}

	if endpoint == "" {
		response.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing Baseten API endpoint",
			"Set endpoint in the provider configuration or use the default endpoint.",
		)
		return
	}

	client, err := baseten.NewClient(apiKey, endpoint)
	if err != nil {
		response.Diagnostics.AddError(
			"Invalid Baseten provider configuration",
			err.Error(),
		)
		return
	}

	response.DataSourceData = client
	response.ResourceData = client
}

func (provider *basetenProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (provider *basetenProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewInstanceTypesDataSource,
	}
}
