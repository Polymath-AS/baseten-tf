package provider

import (
	"context"
	"encoding/json"

	schemavalidator "github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type jsonObjectStringValidator struct{}

func (validator jsonObjectStringValidator) Description(context.Context) string {
	return "value must be a valid JSON object"
}

func (validator jsonObjectStringValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

func (validator jsonObjectStringValidator) ValidateString(ctx context.Context, request schemavalidator.StringRequest, response *schemavalidator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	var value map[string]json.RawMessage
	if err := json.Unmarshal([]byte(request.ConfigValue.ValueString()), &value); err != nil {
		response.Diagnostics.AddAttributeError(
			request.Path,
			"Invalid JSON object",
			"The value must be a valid JSON object. "+err.Error(),
		)
		return
	}

	if value == nil {
		response.Diagnostics.AddAttributeError(
			request.Path,
			"Invalid JSON object",
			"The value must be a JSON object, not null.",
		)
	}
}
