package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"

	stack "github.com/nuonco/nuon/sdks/stack"
)

func TestStackDataSourceSchema(t *testing.T) {
	ctx := context.Background()
	d := NewStackDataSource()
	var resp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema implementation: %+v", diags)
	}
}

func TestFlattenConfigGCP(t *testing.T) {
	cfg := &stack.Config{
		InstallID:     "inst123",
		OrgID:         "org123",
		AppID:         "app123",
		Cloud:         stack.CloudGCP,
		RunnerID:      "runner123",
		RunnerAPIURL:  "https://runner.example.com",
		PhoneHomeURL:  "https://api.example.com/v1/installs/inst123/phone-home/ph123",
		InstallInputs: map[string]string{"domain": "example.com"},
		Secrets: map[string]stack.SecretInput{
			"db_password": {Description: "db", Required: true, Value: "hunter2"},
		},
		GCP: &stack.GCPConfig{
			RunnerInitScriptURL:  "https://init.sh",
			RunnerAPIToken:       "tok",
			ProvisionPermissions: []string{"compute.instances.create"},
			CustomRoles: map[string]stack.GCPRole{
				"extra": {Permissions: []string{"storage.buckets.get"}, Enabled: true},
			},
		},
	}

	var data stackDataSourceModel
	flattenConfig(&data, cfg)

	if data.InstallID.ValueString() != "inst123" {
		t.Errorf("install_id = %q", data.InstallID.ValueString())
	}
	if data.Cloud.ValueString() != "gcp" {
		t.Errorf("cloud = %q", data.Cloud.ValueString())
	}
	if data.PhoneHomeURL.ValueString() != "https://api.example.com/v1/installs/inst123/phone-home/ph123" {
		t.Errorf("phone_home_url = %q", data.PhoneHomeURL.ValueString())
	}
	if data.Secrets["db_password"].Value != "hunter2" {
		t.Errorf("secret value = %q", data.Secrets["db_password"].Value)
	}
	if data.GCP == nil || data.GCP.RunnerAPIToken != "tok" {
		t.Fatalf("gcp not flattened: %+v", data.GCP)
	}
	if r := data.GCP.CustomRoles["extra"]; !r.Enabled || len(r.Permissions) != 1 {
		t.Errorf("custom role = %+v", r)
	}
}

func TestFlattenConfigAWS(t *testing.T) {
	cfg := &stack.Config{
		InstallID:     "inst123",
		OrgID:         "org123",
		AppID:         "app123",
		Cloud:         stack.CloudAWS,
		RunnerID:      "runner123",
		RunnerAPIURL:  "https://runner.example.com",
		PhoneHomeURL:  "https://api.example.com/v1/installs/inst123/phone-home/ph123",
		InstallInputs: map[string]string{"domain": "example.com"},
		Secrets: map[string]stack.SecretInput{
			"db_password": {Description: "db", Required: true, Value: "hunter2"},
		},
		AWS: &stack.AWSConfig{
			Region:                 "us-east-1",
			ClusterName:            "inst123",
			NuonSupportIAMRoleARNs: []string{"arn:aws:iam::123:role/nuon"},
			ProvisionPermissions:   []string{"s3:GetObject"},
			CustomRoles: map[string]stack.RoleConfig{
				"extra": {Permissions: []string{"ec2:DescribeInstances"}, Enabled: true},
			},
		},
	}

	var data stackDataSourceModel
	flattenConfig(&data, cfg)

	if data.Cloud.ValueString() != "aws" {
		t.Errorf("cloud = %q", data.Cloud.ValueString())
	}
	if data.AWS == nil {
		t.Fatal("aws not flattened")
	}
	if data.AWS.Region != "us-east-1" {
		t.Errorf("region = %q", data.AWS.Region)
	}
	if data.AWS.ClusterName != "inst123" {
		t.Errorf("cluster_name = %q", data.AWS.ClusterName)
	}
	if len(data.AWS.NuonSupportIAMRoleARNs) != 1 {
		t.Errorf("nuon_support_iam_role_arns = %+v", data.AWS.NuonSupportIAMRoleARNs)
	}
	if r := data.AWS.CustomRoles["extra"]; !r.Enabled || len(r.Permissions) != 1 {
		t.Errorf("custom role = %+v", r)
	}
	if data.GCP != nil {
		t.Errorf("gcp should be nil for aws config: %+v", data.GCP)
	}
}

// install_inputs is a released map[string]string with no room for per-key metadata,
// so sensitivity arrives as a sibling name list the module can act on.
func TestFlattenConfigSensitiveInputNames(t *testing.T) {
	cfg := &stack.Config{
		Cloud:           stack.CloudAWS,
		InstallInputs:   map[string]string{"domain": "example.com", "api_key": "secret"},
		SensitiveInputs: []string{"api_key"},
		AWS:             &stack.AWSConfig{Region: "us-west-2"},
	}

	var data stackDataSourceModel
	flattenConfig(&data, cfg)

	if len(data.SensitiveInputNames) != 1 || data.SensitiveInputNames[0] != "api_key" {
		t.Errorf("sensitive_input_names = %v", data.SensitiveInputNames)
	}
	if data.InstallInputs["api_key"] != "secret" {
		t.Errorf("sensitive values still travel in install_inputs; got %q", data.InstallInputs["api_key"])
	}
}

// Never null: modules call length()/for_each on it without coalescing.
func TestFlattenConfigSensitiveInputNamesEmpty(t *testing.T) {
	var data stackDataSourceModel
	flattenConfig(&data, &stack.Config{Cloud: stack.CloudAWS})
	if data.SensitiveInputNames == nil {
		t.Error("sensitive_input_names must be empty, not null")
	}
}
