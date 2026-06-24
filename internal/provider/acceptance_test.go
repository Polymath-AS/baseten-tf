package provider

import (
	"os"
	"testing"
)

func TestAccPreflight(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	if os.Getenv("BASETEN_API_KEY") == "" {
		t.Fatal("BASETEN_API_KEY must be set when TF_ACC=1")
	}
}
