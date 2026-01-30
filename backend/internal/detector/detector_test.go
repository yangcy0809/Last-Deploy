package detector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect_PriorityComposeOverDockerfile(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.DeployType != "compose" {
		t.Fatalf("DeployType = %q, want %q", got.DeployType, "compose")
	}
	if got.ComposePath != "docker-compose.yml" {
		t.Fatalf("ComposePath = %q, want %q", got.ComposePath, "docker-compose.yml")
	}
	if strings.TrimSpace(got.ComposeContent) == "" {
		t.Fatalf("ComposeContent empty")
	}
	if len(got.Services) != 1 || got.Services[0] != "web" {
		t.Fatalf("Services = %#v, want %q", got.Services, []string{"web"})
	}
}

func TestDetect_PriorityComposeFileOrder(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services:\n  a: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services:\n  b: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.DeployType != "compose" {
		t.Fatalf("DeployType = %q, want %q", got.DeployType, "compose")
	}
	if got.ComposePath != "compose.yml" {
		t.Fatalf("ComposePath = %q, want %q", got.ComposePath, "compose.yml")
	}
	if len(got.Services) != 1 || got.Services[0] != "b" {
		t.Fatalf("Services = %#v, want %q", got.Services, []string{"b"})
	}
}

func TestDetect_Dockerfile(t *testing.T) {
	dir := t.TempDir()

	const want = "FROM alpine:3.20\n"
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.DeployType != "dockerfile" {
		t.Fatalf("DeployType = %q, want %q", got.DeployType, "dockerfile")
	}
	if got.DockerfilePath != "Dockerfile" {
		t.Fatalf("DockerfilePath = %q, want %q", got.DockerfilePath, "Dockerfile")
	}
	if got.DockerfileContent != want {
		t.Fatalf("DockerfileContent = %q, want %q", got.DockerfileContent, want)
	}
	if got.Services != nil {
		t.Fatalf("Services = %#v, want nil", got.Services)
	}
}

func TestDetect_None_ReturnsDefaultDockerfileTemplate(t *testing.T) {
	dir := t.TempDir()

	got, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.DeployType != "none" {
		t.Fatalf("DeployType = %q, want %q", got.DeployType, "none")
	}
	if got.DockerfilePath != "" {
		t.Fatalf("DockerfilePath = %q, want empty", got.DockerfilePath)
	}
	if got.DockerfileContent != defaultDockerfileTemplate {
		t.Fatalf("DockerfileContent mismatch")
	}
}

func TestDetect_ComposeParseError(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Detect(dir)
	if err == nil {
		t.Fatalf("Detect: expected error")
	}
}

