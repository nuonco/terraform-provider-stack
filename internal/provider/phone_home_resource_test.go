package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPhoneHomeResourceSchema(t *testing.T) {
	ctx := context.Background()
	r := NewPhoneHomeResource()
	var resp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema implementation: %+v", diags)
	}
}

func TestPhoneHomeResourceTypeName(t *testing.T) {
	ctx := context.Background()
	r := NewPhoneHomeResource()
	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "stack"}, &resp)
	if resp.TypeName != "stack_phone_home" {
		t.Errorf("type name = %q, want stack_phone_home", resp.TypeName)
	}
}

// The whole inputs map is sensitive: terraform maps are all-or-nothing, and any
// individual install input may be declared sensitive on the app.
func TestPhoneHomeInputsAttributeIsSensitive(t *testing.T) {
	ctx := context.Background()
	r := NewPhoneHomeResource()
	var resp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resp)

	attr, ok := resp.Schema.Attributes["inputs"]
	if !ok {
		t.Fatal("inputs attribute missing")
	}
	if !attr.IsSensitive() {
		t.Error("inputs must be sensitive")
	}
	if !attr.IsOptional() {
		t.Error("inputs must be optional")
	}
}

func TestInputsFromMap(t *testing.T) {
	ctx := context.Background()

	// Null and unknown both mean "no inputs reported" — the key is then omitted
	// from the payload rather than sent as an empty object.
	for name, m := range map[string]types.Map{
		"null":    types.MapNull(types.StringType),
		"unknown": types.MapUnknown(types.StringType),
	} {
		got, diags := inputsFromMap(ctx, m)
		if diags.HasError() {
			t.Fatalf("%s: diagnostics: %+v", name, diags)
		}
		if len(got) != 0 {
			t.Errorf("%s: got %v, want empty", name, got)
		}
	}

	m, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"domain": types.StringValue("example.com"),
	})
	if diags.HasError() {
		t.Fatalf("map value: %+v", diags)
	}
	got, diags := inputsFromMap(ctx, m)
	if diags.HasError() {
		t.Fatalf("diagnostics: %+v", diags)
	}
	if got["domain"] != "example.com" {
		t.Errorf("domain = %q", got["domain"])
	}
}

// The authenticated phone-home URL is version-invariant, so stack_version_id is
// the only thing that makes a newly generated version show up as a diff.
func TestPhoneHomeSchemaTracksStackVersionID(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewPhoneHomeResource().Schema(context.Background(), resource.SchemaRequest{}, resp)

	attr, ok := resp.Schema.Attributes["stack_version_id"]
	if !ok {
		t.Fatal("stack_version_id attribute missing from stack_phone_home schema")
	}
	if !attr.IsOptional() {
		t.Error("stack_version_id must be optional so existing modules keep working")
	}
	if attr.IsComputed() {
		t.Error("stack_version_id must not be computed, or a new version would not produce a diff")
	}
}
