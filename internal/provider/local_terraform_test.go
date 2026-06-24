package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomModelExamplePlansWithLocalProvider(t *testing.T) {
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform binary not found")
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not found")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	tempDir := t.TempDir()
	providerDir := filepath.Join(tempDir, "provider")
	if err := os.MkdirAll(providerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll provider dir: %v", err)
	}

	providerBinary := filepath.Join(providerDir, "terraform-provider-baseten")
	runCommand(t, repoRoot, nil, goPath, "build", "-o", providerBinary, ".")

	exampleSource := filepath.Join(repoRoot, "examples", "custom-model")
	exampleWorkdir := filepath.Join(tempDir, "custom-model")
	copyDirectory(t, exampleSource, exampleWorkdir)

	terraformConfigPath := filepath.Join(tempDir, "terraformrc")
	terraformConfig := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "registry.terraform.io/polymath-as/baseten" = %q
  }
  direct {}
}
`, providerDir)
	if err := os.WriteFile(terraformConfigPath, []byte(terraformConfig), 0o600); err != nil {
		t.Fatalf("WriteFile terraformrc: %v", err)
	}

	environment := []string{
		"TF_CLI_CONFIG_FILE=" + terraformConfigPath,
		"TF_IN_AUTOMATION=1",
		"BASETEN_API_KEY=test-key",
	}
	runCommand(t, exampleWorkdir, environment, terraformPath, "plan", "-input=false", "-no-color")
}

func runCommand(t *testing.T, workdir string, environment []string, name string, args ...string) {
	t.Helper()

	command := exec.CommandContext(context.Background(), name, args...)
	command.Dir = workdir
	command.Env = append(os.Environ(), environment...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
}

func copyDirectory(t *testing.T, source string, destination string) {
	t.Helper()

	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destination, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		return copyFile(path, targetPath)
	}); err != nil {
		t.Fatalf("copy directory: %v", err)
	}
}

func copyFile(source string, destination string) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := input.Close()
		if returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}

	return closeErr
}
