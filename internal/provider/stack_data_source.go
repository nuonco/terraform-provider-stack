package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	stack "github.com/nuonco/terraform-provider-stack/internal/stack"
)

var (
	_ datasource.DataSource              = (*stackDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*stackDataSource)(nil)
)

// stackDataSource is the stack_config data source. It reads an install stack's
// rendered configuration from the Nuon control plane.
type stackDataSource struct {
	cfg *providerConfig
}

// NewStackDataSource is the data source factory registered with the provider.
func NewStackDataSource() datasource.DataSource {
	return &stackDataSource{}
}

func (d *stackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (d *stackDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cfg, ok := req.ProviderData.(*providerConfig)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data", fmt.Sprintf("expected *providerConfig, got %T", req.ProviderData))
		return
	}
	d.cfg = cfg
}

func (d *stackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	roleAttrs := map[string]schema.Attribute{
		"permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IAM permissions bound to the role's service account."},
		"predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Predefined role bound to the service account, if any."},
		"enabled":         schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the role should be created."},
	}

	awsRoleAttrs := map[string]schema.Attribute{
		"permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IAM action strings granted via an inline policy."},
		"inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON IAM policy document attached as an inline policy. Takes precedence over permissions."},
		"managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs to attach to the role."},
		"enabled":                schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the role should be created."},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Nuon install stack's rendered configuration (runner, permissions, inputs, secrets) from the control plane. Intended for use inside install-stacks modules so the config is read from the API rather than passed in as tfvars.",
		Attributes: map[string]schema.Attribute{
			"phone_home_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Per-stack-version identifier from the Nuon control plane; acts as the secret for this read.",
			},

			"install_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Nuon install ID."},
			"org_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Nuon organization ID."},
			"app_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Nuon application ID."},
			"cloud":          schema.StringAttribute{Computed: true, MarkdownDescription: "Target cloud (aws or gcp)."},
			"runner_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Runner ID for this install."},
			"runner_api_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner API URL the runner reports to."},
			"phone_home_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Phone-home URL the module reports run completion to."},

			"install_inputs": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Customer install-input values.",
			},
			"auto_generate_secrets": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Names of secrets the stack should auto-generate.",
			},
			"secrets": schema.MapNestedAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Customer-supplied secrets, keyed by name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Secret description."},
						"required":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the secret is required."},
						"value":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Secret value."},
					},
				},
			},

			"gcp": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "GCP-specific configuration. Present when cloud is gcp.",
				Attributes: map[string]schema.Attribute{
					"runner_init_script_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Runner bootstrap script URL."},
					"runner_api_token":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Runner API token."},

					"provision_permissions":       schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision service-account permissions."},
					"provision_predefined_role":   schema.StringAttribute{Computed: true, MarkdownDescription: "Provision predefined role, if any."},
					"maintenance_permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance service-account permissions."},
					"maintenance_predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Maintenance predefined role, if any."},
					"deprovision_permissions":     schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision service-account permissions."},
					"deprovision_predefined_role": schema.StringAttribute{Computed: true, MarkdownDescription: "Deprovision predefined role, if any."},

					"break_glass_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Break-glass roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: roleAttrs},
					},
					"custom_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Customer-defined roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: roleAttrs},
					},
				},
			},

			"aws": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "AWS-specific configuration. Present when cloud is aws.",
				Attributes: map[string]schema.Attribute{
					"region":                     schema.StringAttribute{Computed: true, MarkdownDescription: "AWS region the stack is provisioned into."},
					"cluster_name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Resolved EKS cluster-name tag value."},
					"nuon_support_iam_role_arns": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Nuon control-plane IAM role ARNs allowed to assume the operation roles."},

					"provision_permissions":              schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Provision role inline-policy IAM actions."},
					"provision_inline_policy_document":   schema.StringAttribute{Computed: true, MarkdownDescription: "Provision role inline policy document JSON."},
					"provision_managed_policy_arns":      schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the provision role."},
					"maintenance_permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Maintenance role inline-policy IAM actions."},
					"maintenance_inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "Maintenance role inline policy document JSON."},
					"maintenance_managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the maintenance role."},
					"deprovision_permissions":            schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Deprovision role inline-policy IAM actions."},
					"deprovision_inline_policy_document": schema.StringAttribute{Computed: true, MarkdownDescription: "Deprovision role inline policy document JSON."},
					"deprovision_managed_policy_arns":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Managed policy ARNs attached to the deprovision role."},

					"break_glass_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Break-glass roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: awsRoleAttrs},
					},
					"custom_roles": schema.MapNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Customer-defined roles, keyed by name.",
						NestedObject:        schema.NestedAttributeObject{Attributes: awsRoleAttrs},
					},
				},
			},
		},
	}
}

func (d *stackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data stackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := strings.TrimRight(d.cfg.apiURL, "/") + "/v1/stack-runs/" + data.PhoneHomeID.ValueString()
	cfg, err := stack.FetchConfig(ctx, url)
	if err != nil {
		resp.Diagnostics.AddError("fetch stack config failed", err.Error())
		return
	}

	flattenConfig(&data, cfg)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
