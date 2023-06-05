package instatus

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	is "github.com/paydaycay/instatus-client-go"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &componentResource{}
	_ resource.ResourceWithConfigure   = &componentResource{}
	_ resource.ResourceWithImportState = &componentResource{}
)

// Configure adds the provider configured client to the resource.
func (r *componentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*is.Client)
}

// NewComponentResource is a helper function to simplify the provider implementation.
func NewComponentResource() resource.Resource {
	return &componentResource{}
}

// componentResource is the resource implementation.
type componentResource struct {
	client *is.Client
}

// componentResourceModel maps the resource schema data.
type componentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	UniqueEmail types.String `tfsdk:"unique_email"`
	Name        types.String `tfsdk:"name"`
	PageID      types.String `tfsdk:"page_id"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	Order       types.Int64  `tfsdk:"order"`
	GroupID     types.String `tfsdk:"group_id"`
	ShowUptime  types.Bool   `tfsdk:"show_uptime"`
	Grouped     types.Bool   `tfsdk:"grouped"`
	Group       types.String `tfsdk:"group"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

// Metadata returns the resource type name.
func (r *componentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_component"
}

// Schema defines the schema for the resource.
func (r *componentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	componentStatuses := []string{"OPERATIONAL", "UNDERMAINTENANCE", "DEGRADEDPERFORMANCE", "PARTIALOUTAGE", "MAJOROUTAGE"}

	resp.Schema = schema.Schema{
		Description: "Manages a component.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "String Identifier of the component.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"unique_email": schema.StringAttribute{
				Description: "Unique email generated by Instatus for the component.",
				Computed:    true,
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update of the component.",
				Computed:    true,
			},
			"page_id": schema.StringAttribute{
				Description: "String Identifier of the page of the component.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the component.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the component.",
				Optional:    true,
			},
			"status": schema.StringAttribute{
				Description: fmt.Sprintf("Status of the component. One of: (%s).", strings.Join(componentStatuses, ", ")),
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf(componentStatuses...)},
			},
			"order": schema.Int64Attribute{
				Description: "Order in the page of the component.",
				Optional:    true,
				Computed:    true,
			},
			"group_id": schema.StringAttribute{
				Description: "String Identifier of the parent group of the component.",
				Computed:    true,
			},
			"show_uptime": schema.BoolAttribute{
				Description: "Whether show uptime is enabled in the component.",
				Optional:    true,
			},
			"grouped": schema.BoolAttribute{
				Description: "Whether the component is in a group (Require group set to desired name when true).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"group": schema.StringAttribute{
				Description: "Name of the group for the component (Require grouped set to true).",
				Optional:    true,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *componentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan componentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var item is.Component = is.Component{
		Name:        plan.Name.ValueStringPointer(),
		Description: plan.Description.ValueStringPointer(),
		Status:      plan.Status.ValueStringPointer(),
		Order:       plan.Order.ValueInt64Pointer(),
		ShowUptime:  plan.ShowUptime.ValueBoolPointer(),
		Grouped:     plan.Grouped.ValueBoolPointer(),
		Group:       plan.Group.ValueStringPointer(),
	}

	// Create new component
	component, err := r.client.CreateComponent(plan.PageID.ValueString(), &item)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating component",
			"Could not create component, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringPointerValue(component.ID)
	plan.UniqueEmail = types.StringPointerValue(component.UniqueEmail)
	plan.Order = types.Int64PointerValue(component.Order)
	plan.GroupID = types.StringPointerValue(component.GroupID)
	plan.Group = types.StringPointerValue(component.Group.Name)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *componentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state componentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed component value from Instatus
	component, err := r.client.GetComponent(state.PageID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Instatus Component",
			"Could not read Instatus component ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}
	// Overwrite items with refreshed state
	state.UniqueEmail = types.StringPointerValue(component.UniqueEmail)
	state.Name = types.StringPointerValue(component.Name)
	state.Description = types.StringPointerValue(component.Description)
	state.Status = types.StringPointerValue(component.Status)
	state.Order = types.Int64PointerValue(component.Order)
	state.GroupID = types.StringPointerValue(component.GroupID)
	state.ShowUptime = types.BoolPointerValue(component.ShowUptime)
	state.Grouped = types.BoolValue(component.Group.Name != nil)
	state.Group = types.StringPointerValue(component.Group.Name)

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *componentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan componentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var item is.Component = is.Component{
		Name:        plan.Name.ValueStringPointer(),
		Description: plan.Description.ValueStringPointer(),
		Status:      plan.Status.ValueStringPointer(),
		Order:       plan.Order.ValueInt64Pointer(),
		ShowUptime:  plan.ShowUptime.ValueBoolPointer(),
		Grouped:     plan.Grouped.ValueBoolPointer(),
		Group:       plan.Group.ValueStringPointer(),
	}

	// Update existing component
	component, err := r.client.UpdateComponent(plan.PageID.ValueString(), plan.ID.ValueString(), &item)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Instatus Component",
			"Could not update component, unexpected error: "+err.Error(),
		)
		return
	}

	resp.Diagnostics.AddWarning("Group name : "+types.StringPointerValue(component.Group.Name).ValueString(), types.StringPointerValue(component.Group.Name).ValueString())
	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringPointerValue(component.ID)
	plan.GroupID = types.StringPointerValue(component.GroupID)
	plan.Group = types.StringPointerValue(component.Group.Name)
	plan.Order = types.Int64PointerValue(component.Order)
	plan.UniqueEmail = types.StringPointerValue(component.UniqueEmail)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *componentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state componentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing component
	err := r.client.DeleteComponent(state.PageID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Instatus Component",
			"Could not delete component, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *componentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
