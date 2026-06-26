package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAccPreflight(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	if os.Getenv("BASETEN_API_KEY") == "" {
		t.Fatal("BASETEN_API_KEY must be set when TF_ACC=1")
	}
}

func TestAccCustomModelScaleToZeroLifecycle(t *testing.T) {
	skipUnlessAcceptance(t)

	tempDir := t.TempDir()
	repoRoot := repositoryRoot(t)
	terraformPath := executablePath(t, "terraform")
	providerDir := buildLocalProvider(t, tempDir, repoRoot)
	terraformConfigPath := writeTerraformCLIConfig(t, tempDir, providerDir)
	environment := localProviderEnvironment(terraformConfigPath, os.Getenv("BASETEN_API_KEY"))

	workdir := filepath.Join(tempDir, "custom-model-acceptance")
	modelName := fmt.Sprintf("baseten-tf-acc-%d", time.Now().UnixNano())
	writeAcceptanceFixture(t, workdir, modelName, 1)

	t.Cleanup(func() {
		runCommand(t, workdir, environment, terraformPath, "destroy", "-auto-approve", "-input=false", "-no-color")
	})

	runCommand(t, workdir, environment, terraformPath, "apply", "-auto-approve", "-input=false", "-no-color")
	state := runCommandOutput(t, workdir, environment, terraformPath, "state", "show", "-no-color", "baseten_custom_model.acceptance")
	assertStateContains(t, state, "min_replica = 0")
	assertStateContains(t, state, "max_replica = 1")
	assertStateContains(t, state, "model_id")
	assertStateContains(t, state, "deployment_id")

	writeAcceptanceFixture(t, workdir, modelName, 2)
	runCommand(t, workdir, environment, terraformPath, "apply", "-auto-approve", "-input=false", "-no-color")
	updatedState := runCommandOutput(t, workdir, environment, terraformPath, "state", "show", "-no-color", "baseten_custom_model.acceptance")
	assertStateContains(t, updatedState, "min_replica = 0")
	assertStateContains(t, updatedState, "max_replica = 2")
}

func skipUnlessAcceptance(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	if os.Getenv("BASETEN_API_KEY") == "" {
		t.Fatal("BASETEN_API_KEY must be set when TF_ACC=1")
	}
}

func writeAcceptanceFixture(t *testing.T, workdir string, modelName string, maxReplica int64) {
	t.Helper()

	accelerator := os.Getenv("BASETEN_ACC_ACCELERATOR")
	if accelerator == "" {
		accelerator = "A10G"
	}

	writeFile(t, filepath.Join(workdir, "main.tf"), acceptanceTerraformConfig(modelName, maxReplica))
	writeFile(t, filepath.Join(workdir, "model", "config.yaml"), acceptanceModelConfig(modelName, accelerator))
	writeFile(t, filepath.Join(workdir, "model", "model", "model.py"), acceptanceModelCode())
}

func acceptanceTerraformConfig(modelName string, maxReplica int64) string {
	return fmt.Sprintf(`terraform {
  required_providers {
    baseten = {
      source = "polymath-as/baseten"
    }
  }
}

provider "baseten" {}

locals {
  model_path  = "${path.module}/model"
  model_files = sort(fileset(local.model_path, "**"))
  model_hash = sha256(join("", [
    for file in local.model_files : filesha256("${local.model_path}/${file}")
  ]))
}

resource "baseten_custom_model" "acceptance" {
  name        = %[1]q
  source_path = local.model_path
  source_hash = local.model_hash

  raw_config  = file("${local.model_path}/config.yaml")
  config_json = jsonencode(yamldecode(file("${local.model_path}/config.yaml")))

  min_replica        = 0
  max_replica        = %[2]d
  scale_down_delay   = 60
  concurrency_target = 1

  timeouts {
    create = "90m"
    update = "10m"
    delete = "10m"
  }
}
`, modelName, maxReplica)
}

func acceptanceModelConfig(modelName string, accelerator string) string {
	return fmt.Sprintf(`model_name: %s
python_version: py311
requirements: []
resources:
  accelerator: %s
`, modelName, accelerator)
}

func acceptanceModelCode() string {
	return `class Model:
    def load(self):
        pass

    def predict(self, request):
        return {"ok": True, "request": request}
`
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func assertStateContains(t *testing.T, state string, expected string) {
	t.Helper()

	if !strings.Contains(state, expected) {
		t.Fatalf("state does not contain %q:\n%s", expected, state)
	}
}
