// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lahabana/terraform-provider-kuma/internal/kumaapi"
	"strings"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &KumaRawResource{}
var _ resource.ResourceWithImportState = &KumaRawResource{}
var _ resource.ResourceWithModifyPlan = &KumaRawResource{}

func NewKumaMeshedResource() resource.Resource {
	return &KumaRawResource{}
}

// KumaRawResource defines the resource implementation.
type KumaRawResource struct {
	client   kumaapi.Client
	metadata kumaapi.Metadata
}

// KumaMeshedResourceModel describes the resource data model.
type KumaMeshedResourceModel struct {
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Mesh     types.String `tfsdk:"mesh"`
	JsonBody types.String `tfsdk:"json_body"`
}

func (r *KumaRawResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_raw_resource"
}

func (r *KumaRawResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip when deleting
	if req.Plan.Raw.IsNull() {
		return
	}
	// Do nothing if there is no state value.
	plan := KumaMeshedResourceModel{}
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !plan.Name.IsUnknown() {
		return
	}

	meta := map[string]interface{}{}
	if err := json.Unmarshal([]byte(plan.JsonBody.ValueString()), &meta); err != nil {
		resp.Diagnostics.AddError("failed extracting meta", fmt.Sprintf("json parse failed, error: %s", err))
		return
	}
	if v, ok := meta["name"]; ok {
		plan.Name = types.StringValue(v.(string))
	}
	if v, ok := meta["type"]; ok {
		plan.Type = types.StringValue(v.(string))
	}
	if v, ok := meta["mesh"]; ok {
		plan.Mesh = types.StringValue(v.(string))
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *KumaRawResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Kuma resource",

		Attributes: map[string]schema.Attribute{
			// TODO can we have a derived field to have users just use yaml?
			"json_body": schema.StringAttribute{
				MarkdownDescription: "The entity as you would have created it in json format `kumactl apply -f`",
				Optional:            true,
				Computed:            true,
			},
			"mesh": schema.StringAttribute{
				MarkdownDescription: "The mesh the resource is part of, if unset it uses `json_body` to extract it",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the resource, if unset it uses `json_body` to extract it",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The type of the resource, if unset it uses `json_body` to extract it",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *KumaRawResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(kumaapi.Client)
	metadata, err := client.HeartBeat(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed to heartbeat control-plane", err.Error())

	}

	tflog.Info(ctx, "successfully checked connection", map[string]interface{}{"info": metadata})

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
	r.metadata = metadata
}

func (r *KumaRawResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KumaMeshedResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	resourcePath := r.metadata.PathForResource(data.Type.ValueString())
	if resourcePath == "" {
		resp.Diagnostics.AddError("unsupported resource type", fmt.Sprintf("Resource type '%s' is not supported by the server", data.Type.ValueString()))
		return
	}

	if resp.Diagnostics.HasError() {
		return
	}
	res, err := r.client.FetchResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to fetch resource have create, got error: %s", err))
		return
	}
	if res != nil {
		resp.Diagnostics.AddError("Unable to Create Resource", "Resource already exists!")
		return
	}

	err = r.client.PutResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString(), data.JsonBody.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to create resource, got error: %s", err))
		return
	}
	res, err = r.client.FetchResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to fetch resource after create, got error: %s", err))
		return
	}
	if res == nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Resource didn't exist just after the put, got error: %s", err))
		return
	}
	out, err := removeTimes(res)
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Failed to normalize resource, get error: %s", err))
		return
	}
	data.JsonBody = types.StringValue(string(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func removeTimes(data []byte) ([]byte, error) {
	m := map[string]interface{}{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("fail unmarshalling: %w", err)
	}
	delete(m, "creationTime")
	delete(m, "modificationTime")

	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("fail marshalling: %w", err)
	}
	return out, nil
}

func (r *KumaRawResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KumaMeshedResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resourcePath := r.metadata.PathForResource(data.Type.ValueString())
	if resourcePath == "" {
		resp.Diagnostics.AddError("unsupported resource type", fmt.Sprintf("Resource type '%s' is not supported by the server", data.Type.ValueString()))
		return
	}
	res, err := r.client.FetchResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to read policy, got error: %s", err))
		return
	}
	if res == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	out, err := removeTimes(res)
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Failed to normalize resource, get error: %s", err))
		return
	}
	data.JsonBody = types.StringValue(string(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KumaRawResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data KumaMeshedResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourcePath := r.metadata.PathForResource(data.Type.ValueString())
	if resourcePath == "" {
		resp.Diagnostics.AddError("unsupported resource type", fmt.Sprintf("Resource type '%s' is not supported by the server", data.Type.ValueString()))
		return
	}
	err := r.client.PutResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString(), data.JsonBody.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to create resource, got error: %s", err))
		return
	}
	res, err := r.client.FetchResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to fetch resource after create, got error: %s", err))
		return
	}
	if res == nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Resource didn't exist just after the put, got error: %s", err))
		return
	}
	out, err := removeTimes(res)
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Failed to normalize resource, get error: %s", err))
		return
	}
	data.JsonBody = types.StringValue(string(out))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KumaRawResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KumaMeshedResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resourcePath := r.metadata.PathForResource(data.Type.ValueString())
	if resourcePath == "" {
		resp.Diagnostics.AddError("unsupported resource type", fmt.Sprintf("Resource type '%s' is not supported by the server", data.Type.ValueString()))
		return
	}
	out, err := r.client.FetchResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("client Error", fmt.Sprintf("Unable to read policy, got error: %s", err))
		return
	}
	if out == nil {
		resp.Diagnostics.AddWarning("already deleted", "Resource was already deleted")
		return
	}

	err = r.client.DeleteResource(ctx, data.Mesh.ValueString(), resourcePath, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("delete error", fmt.Sprintf("Unable to delete policy, got error: %s", err))
		return
	}
}

func (r *KumaRawResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(strings.Trim(req.ID, "/"), "/")
	if len(parts) == 2 {
		parts = append([]string{""}, parts...)
	}
	resourceName := r.metadata.ResourceForPath(parts[1])
	if resourceName != "" {
		parts[1] = resourceName
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("mesh"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[1])...)
}
