// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lahabana/terraform-provider-kuma/internal/kumaapi"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure KumaProvider satisfies various provider interfaces.
var _ provider.Provider = &KumaProvider{}

// KumaProvider defines the provider implementation.
type KumaProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// KumaProviderModel describes the provider data model.
type KumaProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *KumaProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kuma"
	resp.Version = p.version
}

func (p *KumaProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Endpoint to the Global and Standalone Control-plane to use",
				Optional:            false,
				Required:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Optional token if token is enabled",
				Optional:            true,
				Required:            false,
				Sensitive:           true,
			},
		},
	}
}

func (p *KumaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KumaProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if data.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown Kuma cp endpoint",
			"The provider cannot create the Kuma API client as there is an unknown configuration value for the Kuma endpoint. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the KUMA_ENDPOINT environment variable.",
		)
	}

	if data.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Kuma cp token",
			"The provider cannot create the Kuma API client as there is an unknown configuration value for the Kuma token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the KUMA_TOKEN environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("KUMA_ENDPOINT")
	// Configuration values are now available.
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}

	token := os.Getenv("KUMA_TOKEN")
	if !data.Token.IsNull() {
		token = data.Token.ValueString()
	}

	// Example client configuration for data sources and resources
	client := kumaapi.NewClient(endpoint, token)

	index, err := client.HeartBeat(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed to heartbeat control-plane", err.Error())

	}

	tflog.Info(ctx, "successfully checked connection", map[string]interface{}{"info": index})
	if resp.Diagnostics.HasError() {
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *KumaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewKumaMeshedResource,
	}
}

func (p *KumaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KumaProvider{
			version: version,
		}
	}
}
