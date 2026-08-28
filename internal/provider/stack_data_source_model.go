package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuonco/nuon/sdks/stack/models"
)

// stackDataSourceModel is the Terraform shape for the stack_config data source. It
// mirrors the install-stack config the control plane renders (the same data the
// legacy flow wrote to tfvars), so an install-stacks module can read it from the
// API instead of receiving it as variables.
type stackDataSourceModel struct {
	InstallID    types.String `tfsdk:"install_id"`
	OrgID        types.String `tfsdk:"org_id"`
	AppID        types.String `tfsdk:"app_id"`
	Cloud        types.String `tfsdk:"cloud"`
	RunnerID     types.String `tfsdk:"runner_id"`
	RunnerAPIURL types.String `tfsdk:"runner_api_url"`
	PhoneHomeURL types.String `tfsdk:"phone_home_url"`

	InstallInputs       map[string]string   `tfsdk:"install_inputs"`
	RequiredInputNames  []string            `tfsdk:"required_input_names"`
	SensitiveInputNames []string            `tfsdk:"sensitive_input_names"`
	AutoGenerateSecrets []string            `tfsdk:"auto_generate_secrets"`
	Secrets             map[string]secretTF `tfsdk:"secrets"`

	GCP *gcpTF `tfsdk:"gcp"`
	AWS *awsTF `tfsdk:"aws"`
}

// secretTF mirrors the module's secrets map(object) element.
type secretTF struct {
	Description string `tfsdk:"description"`
	Required    bool   `tfsdk:"required"`
	Value       string `tfsdk:"value"`
}

// gcpRoleTF mirrors the module's break_glass_roles / custom_roles element.
type gcpRoleTF struct {
	Permissions    []string            `tfsdk:"permissions"`
	Policies       map[string][]string `tfsdk:"policies"`
	PredefinedRole string              `tfsdk:"predefined_role"`
	Enabled        bool                `tfsdk:"enabled"`
}

// gcpTF carries the GCP-specific install-stack config.
type gcpTF struct {
	// The install's GCP target. Both may be empty: a GCP install can be created
	// without a project/region and have them recorded by the first provision, so
	// the module treats these as a base its own variables can override rather
	// than as guaranteed values. The AWS counterpart (awsTF.Region) is always
	// set, which is why only GCP needs the override path.
	ProjectID string `tfsdk:"project_id"`
	Region    string `tfsdk:"region"`

	RunnerInitScriptURL string `tfsdk:"runner_init_script_url"`
	RunnerAPIToken      string `tfsdk:"runner_api_token"`
	RunnerMachineType   string `tfsdk:"runner_machine_type"`

	ProvisionPermissions      []string `tfsdk:"provision_permissions"`
	ProvisionPredefinedRole   string   `tfsdk:"provision_predefined_role"`
	MaintenancePermissions    []string `tfsdk:"maintenance_permissions"`
	MaintenancePredefinedRole string   `tfsdk:"maintenance_predefined_role"`
	DeprovisionPermissions    []string `tfsdk:"deprovision_permissions"`
	DeprovisionPredefinedRole string   `tfsdk:"deprovision_predefined_role"`

	ProvisionPolicies   map[string][]string `tfsdk:"provision_policies"`
	MaintenancePolicies map[string][]string `tfsdk:"maintenance_policies"`
	DeprovisionPolicies map[string][]string `tfsdk:"deprovision_policies"`

	BreakGlassRoles map[string]gcpRoleTF `tfsdk:"break_glass_roles"`
	CustomRoles     map[string]gcpRoleTF `tfsdk:"custom_roles"`
}

// awsRoleTF mirrors the module's break_glass_roles / custom_roles element.
type awsRoleTF struct {
	Permissions          []string `tfsdk:"permissions"`
	InlinePolicyDocument string   `tfsdk:"inline_policy_document"`
	ManagedPolicyARNs    []string `tfsdk:"managed_policy_arns"`
	Enabled              bool     `tfsdk:"enabled"`
}

// awsTF carries the AWS-specific install-stack config.
type awsTF struct {
	Region                 string   `tfsdk:"region"`
	ClusterName            string   `tfsdk:"cluster_name"`
	RunnerMachineType      string   `tfsdk:"runner_machine_type"`
	NuonSupportIAMRoleARNs []string `tfsdk:"nuon_support_iam_role_arns"`

	ProvisionPermissions            []string `tfsdk:"provision_permissions"`
	ProvisionInlinePolicyDocument   string   `tfsdk:"provision_inline_policy_document"`
	ProvisionManagedPolicyARNs      []string `tfsdk:"provision_managed_policy_arns"`
	MaintenancePermissions          []string `tfsdk:"maintenance_permissions"`
	MaintenanceInlinePolicyDocument string   `tfsdk:"maintenance_inline_policy_document"`
	MaintenanceManagedPolicyARNs    []string `tfsdk:"maintenance_managed_policy_arns"`
	DeprovisionPermissions          []string `tfsdk:"deprovision_permissions"`
	DeprovisionInlinePolicyDocument string   `tfsdk:"deprovision_inline_policy_document"`
	DeprovisionManagedPolicyARNs    []string `tfsdk:"deprovision_managed_policy_arns"`

	BreakGlassRoles map[string]awsRoleTF `tfsdk:"break_glass_roles"`
	CustomRoles     map[string]awsRoleTF `tfsdk:"custom_roles"`
}

// flattenConfig copies the fetched SDK config onto the data source model. install_id
// is the caller's input and is echoed back from the response, which the control plane
// resolves to the same value.
func flattenConfig(data *stackDataSourceModel, cfg *models.AppInstallerSDKConfig) {
	data.InstallID = types.StringValue(cfg.InstallID)
	data.OrgID = types.StringValue(cfg.OrgID)
	data.AppID = types.StringValue(cfg.AppID)
	data.Cloud = types.StringValue(string(cfg.Cloud))
	data.RunnerID = types.StringValue(cfg.RunnerID)
	data.RunnerAPIURL = types.StringValue(cfg.RunnerAPIURL)
	data.PhoneHomeURL = types.StringValue(cfg.PhoneHomeURL)

	// Collections are emitted as empty (never null) so module authors can call
	// length()/for_each on them without coalescing — matching the contract the
	// legacy tfvars flow provided via variable defaults.
	data.InstallInputs = orEmptyMap(cfg.InstallInputs)
	data.RequiredInputNames = orEmptySlice(cfg.RequiredInputs)
	data.SensitiveInputNames = orEmptySlice(cfg.SensitiveInputs)
	data.AutoGenerateSecrets = orEmptySlice(cfg.AutoGenerateSecrets)

	data.Secrets = make(map[string]secretTF, len(cfg.Secrets))
	for name, s := range cfg.Secrets {
		data.Secrets[name] = secretTF{
			Description: s.Description,
			Required:    s.Required,
			Value:       s.Value,
		}
	}

	if cfg.Gcp != nil {
		data.GCP = flattenGCP(cfg.Gcp)
	}
	if cfg.Aws != nil {
		data.AWS = flattenAWS(cfg.Aws)
	}
}

func flattenAWS(a *models.AppInstallerSDKAWSConfig) *awsTF {
	return &awsTF{
		Region:                          a.Region,
		ClusterName:                     a.ClusterName,
		RunnerMachineType:               a.RunnerMachineType,
		NuonSupportIAMRoleARNs:          orEmptySlice(a.NuonSupportIamRoleArns),
		ProvisionPermissions:            orEmptySlice(a.ProvisionPermissions),
		ProvisionInlinePolicyDocument:   a.ProvisionInlinePolicyDocument,
		ProvisionManagedPolicyARNs:      orEmptySlice(a.ProvisionManagedPolicyArns),
		MaintenancePermissions:          orEmptySlice(a.MaintenancePermissions),
		MaintenanceInlinePolicyDocument: a.MaintenanceInlinePolicyDocument,
		MaintenanceManagedPolicyARNs:    orEmptySlice(a.MaintenanceManagedPolicyArns),
		DeprovisionPermissions:          orEmptySlice(a.DeprovisionPermissions),
		DeprovisionInlinePolicyDocument: a.DeprovisionInlinePolicyDocument,
		DeprovisionManagedPolicyARNs:    orEmptySlice(a.DeprovisionManagedPolicyArns),
		BreakGlassRoles:                 flattenAWSRoles(a.BreakGlassRoles),
		CustomRoles:                     flattenAWSRoles(a.CustomRoles),
	}
}

func flattenAWSRoles(in map[string]models.AppInstallerSDKRoleConfig) map[string]awsRoleTF {
	out := make(map[string]awsRoleTF, len(in))
	for name, r := range in {
		out[name] = awsRoleTF{
			Permissions:          orEmptySlice(r.Permissions),
			InlinePolicyDocument: r.InlinePolicyDocument,
			ManagedPolicyARNs:    orEmptySlice(r.ManagedPolicyArns),
			Enabled:              r.Enabled,
		}
	}
	return out
}

func flattenGCP(g *models.AppInstallerSDKGCPConfig) *gcpTF {
	return &gcpTF{
		ProjectID:                 g.ProjectID,
		Region:                    g.Region,
		RunnerInitScriptURL:       g.RunnerInitScriptURL,
		RunnerAPIToken:            g.RunnerAPIToken,
		RunnerMachineType:         g.RunnerMachineType,
		ProvisionPermissions:      orEmptySlice(g.ProvisionPermissions),
		ProvisionPredefinedRole:   g.ProvisionPredefinedRole,
		MaintenancePermissions:    orEmptySlice(g.MaintenancePermissions),
		MaintenancePredefinedRole: g.MaintenancePredefinedRole,
		DeprovisionPermissions:    orEmptySlice(g.DeprovisionPermissions),
		DeprovisionPredefinedRole: g.DeprovisionPredefinedRole,
		ProvisionPolicies:         orEmptyMapList(g.ProvisionPolicies),
		MaintenancePolicies:       orEmptyMapList(g.MaintenancePolicies),
		DeprovisionPolicies:       orEmptyMapList(g.DeprovisionPolicies),
		BreakGlassRoles:           flattenGCPRoles(g.BreakGlassRoles),
		CustomRoles:               flattenGCPRoles(g.CustomRoles),
	}
}

func flattenGCPRoles(in map[string]models.AppInstallerSDKGCPRole) map[string]gcpRoleTF {
	out := make(map[string]gcpRoleTF, len(in))
	for name, r := range in {
		out[name] = gcpRoleTF{
			Permissions:    orEmptySlice(r.Permissions),
			Policies:       orEmptyMapList(r.Policies),
			PredefinedRole: r.PredefinedRole,
			Enabled:        r.Enabled,
		}
	}
	return out
}

func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func orEmptyMapList(m map[string][]string) map[string][]string {
	if m == nil {
		return map[string][]string{}
	}
	return m
}
