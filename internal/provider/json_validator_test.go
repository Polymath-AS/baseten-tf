package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	schemavalidator "github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestJSONConfigValidatorAcceptsObjects(t *testing.T) {
	validator := jsonObjectStringValidator{}
	request := schemavalidator.StringRequest{
		Path:        path.Root("config_json"),
		ConfigValue: types.StringValue(`{"model_name":"demo"}`),
	}
	response := schemavalidator.StringResponse{}

	validator.ValidateString(context.Background(), request, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("ValidateString returned errors: %v", response.Diagnostics)
	}
}

func TestJSONConfigValidatorRejectsNonObjects(t *testing.T) {
	validator := jsonObjectStringValidator{}
	request := schemavalidator.StringRequest{
		Path:        path.Root("config_json"),
		ConfigValue: types.StringValue(`null`),
	}
	response := schemavalidator.StringResponse{}

	validator.ValidateString(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("ValidateString accepted null")
	}
}
