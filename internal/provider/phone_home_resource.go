package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	stack "github.com/nuonco/nuon/sdks/stack"
)

const (
	phoneHomeRequestCreate = "Create"
	phoneHomeRequestUpdate = "Update"
	phoneHomeRequestDelete = "Delete"
)

var (
	_ resource.Resource              = (*phoneHomeResource)(nil)
	_ resource.ResourceWithConfigure = (*phoneHomeResource)(nil)
)

// phoneHomeResource is the stack_phone_home resource. It reports the result of
// an install-stack run to the Nuon control plane over the public phone-home
// endpoint, so an install-stacks module can report run status through the
// provider instead of constructing the HTTP request itself. The resource
// lifecycle drives the request_type: Create on first apply, Update when the
// reported outputs change, Delete on destroy.
type phoneHomeResource struct {
	cfg *providerConfig
}

// NewPhoneHomeResource is the resource factory registered with the provider.
func NewPhoneHomeResource() resource.Resource {
	return &phoneHomeResource{}
}

func (r *phoneHomeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_phone_home"
}

func (r *phoneHomeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cfg, ok := req.ProviderData.(*providerConfig)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("expected *providerConfig, got %T", req.ProviderData))
		return
	}
	r.cfg = cfg
}

func (r *phoneHomeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reports the result of an install-stack run to the Nuon control plane. Create/update/destroy map to the phone-home request_type (Create/Update/Delete). Intended for use inside install-stacks modules so run status is reported through the provider rather than a hand-rolled HTTP call.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier, the install ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"install_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nuon install ID (URL path).",
			},
			"phone_home_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Phone-home endpoint for this stack version, read from `stack_config.phone_home_url`. Sourced from the API rather than configured by hand: it embeds a per-stack-version identifier the caller has no other way to know.",
			},
			"phone_home_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Target cloud for the report (`aws`, `azure`, or `gcp`). Merged into the payload as `phone_home_type`.",
			},
			"payload": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The phone-home body as a JSON object string (typically `jsonencode({...})`). The provider injects `request_type`, `phone_home_type` and `inputs`; any values for those keys in the payload are overwritten.",
			},
			"inputs": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				// A terraform map is sensitive as a whole or not at all, and any
				// individual install input may be declared sensitive on the app —
				// so the map is marked sensitive, matching how the data source
				// treats its whole `secrets` map.
				Sensitive:           true,
				MarkdownDescription: "Install-input values this stack resolved, sent as the `inputs` object. The control plane merges them over the install's current inputs and makes the result the install's inputs, so a module's tfvars becomes a way to set input values. Every key must be a customer-source app input; anything else is rejected.",
			},
			"stack_version_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ID of the stack version being reported, from `stack_config.stack_version_id`. Never sent in the body — it exists so that generating a new version changes this resource and Terraform re-reports. The authenticated phone-home URL is identical for every version, so without it an applied stack with a new version pending shows no diff and the install's await step waits forever.",
			},
		},
	}
}

func (r *phoneHomeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data phoneHomeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.report(ctx, &data, phoneHomeRequestCreate); err != nil {
		resp.Diagnostics.AddError("phone home failed", err.Error())
		return
	}
	data.ID = types.StringValue(data.InstallID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *phoneHomeResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Write-only endpoint: there is no GET to refresh against, so state is
	// preserved as-is.
}

func (r *phoneHomeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data phoneHomeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.report(ctx, &data, phoneHomeRequestUpdate); err != nil {
		resp.Diagnostics.AddError("phone home failed", err.Error())
		return
	}
	data.ID = types.StringValue(data.InstallID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *phoneHomeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data phoneHomeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Best-effort: ctl-api short-circuits Delete, and a failed report must not
	// wedge terraform destroy.
	if err := r.report(ctx, &data, phoneHomeRequestDelete); err != nil {
		resp.Diagnostics.AddWarning("phone home delete failed", err.Error())
	}
}

// inputsFromMap converts the resource's inputs attribute to the payload object.
// Null and unknown both mean "no inputs reported".
func inputsFromMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	out := make(map[string]string, len(m.Elements()))
	diags := m.ElementsAs(ctx, &out, false)
	return out, diags
}

func (r *phoneHomeResource) report(ctx context.Context, data *phoneHomeResourceModel, requestType string) error {
	payload := map[string]any{}
	if raw := data.Payload.ValueString(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return fmt.Errorf("payload must be a JSON object: %w", err)
		}
	}
	payload["request_type"] = requestType
	payload["phone_home_type"] = data.PhoneHomeType.ValueString()

	// Omitted entirely when unset: an empty object would be a report that the stack
	// resolved no inputs, which is not the same as not reporting inputs at all.
	inputs, diags := inputsFromMap(ctx, data.Inputs)
	if diags.HasError() {
		return fmt.Errorf("inputs must be a map of strings: %v", diags.Errors())
	}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}

	return stack.PhoneHome(ctx, stack.Options{
		APIURL:    r.cfg.apiURL,
		InstallID: data.InstallID.ValueString(),
		APIToken:  r.cfg.apiToken,
		OrgID:     r.cfg.orgID,
	}, data.PhoneHomeURL.ValueString(), payload)
}
