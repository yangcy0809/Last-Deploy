package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type DeployType string

const (
	DeployTypeDockerfile DeployType = "dockerfile"
	DeployTypeCompose    DeployType = "compose"
)

func ResolveDeployType(deployType, composeFile string) DeployType {
	switch strings.ToLower(strings.TrimSpace(deployType)) {
	case string(DeployTypeCompose):
		return DeployTypeCompose
	case string(DeployTypeDockerfile):
		return DeployTypeDockerfile
	default:
		if strings.TrimSpace(composeFile) != "" {
			return DeployTypeCompose
		}
		return DeployTypeDockerfile
	}
}

type ComposeSpec struct {
	ProjectID      string
	WorkDir        string
	HostWorkDir    string
	ComposeFile    string
	ComposeService string
}

var composeServiceRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func ComposeUp(ctx context.Context, spec ComposeSpec) error {
	return runComposeUpStop(ctx, spec, "up", "-d")
}

func ComposeStop(ctx context.Context, spec ComposeSpec) error {
	return runComposeUpStop(ctx, spec, "stop")
}

func ComposePause(ctx context.Context, spec ComposeSpec) error {
	return runComposeUpStop(ctx, spec, "pause")
}

func ComposeUnpause(ctx context.Context, spec ComposeSpec) error {
	return runComposeUpStop(ctx, spec, "unpause")
}

func ComposeDown(ctx context.Context, spec ComposeSpec) error {
	if spec.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if spec.WorkDir == "" {
		return fmt.Errorf("work dir is required")
	}
	if strings.TrimSpace(spec.ComposeFile) == "" {
		return fmt.Errorf("compose_file is required")
	}

	composeFile := normalizeComposeFile(spec.ComposeFile, spec.ProjectID)
	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(spec.WorkDir, filepath.FromSlash(composeFile))
	}

	projectName := "last-deploy-" + spec.ProjectID
	cmdArgs := []string{"compose", "-p", projectName, "-f", composeFile, "down", "--remove-orphans"}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Dir = spec.WorkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func parseComposeServices(serviceStr string) []string {
	serviceStr = strings.TrimSpace(serviceStr)
	if serviceStr == "" {
		return nil
	}
	var services []string
	for _, s := range strings.Split(serviceStr, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			services = append(services, s)
		}
	}
	return services
}

func runComposeUpStop(ctx context.Context, spec ComposeSpec, args ...string) error {
	if spec.ProjectID == "" {
		return fmt.Errorf("project id is required")
	}
	if spec.WorkDir == "" {
		return fmt.Errorf("work dir is required")
	}
	if strings.TrimSpace(spec.ComposeFile) == "" {
		return fmt.Errorf("compose_file is required")
	}

	services := parseComposeServices(spec.ComposeService)
	for _, svc := range services {
		if !composeServiceRe.MatchString(svc) {
			return fmt.Errorf("invalid compose_service: %s", svc)
		}
	}

	composeFile := normalizeComposeFile(spec.ComposeFile, spec.ProjectID)
	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(spec.WorkDir, filepath.FromSlash(composeFile))
	}

	projectName := "last-deploy-" + spec.ProjectID
	cmdArgs := []string{"compose", "-p", projectName, "-f", composeFile}

	if len(services) > 0 {
		override, err := writeComposeOverride(spec.ProjectID, services)
		if err != nil {
			return err
		}
		defer os.Remove(override)
		cmdArgs = append(cmdArgs, "-f", override)
	}

	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, services...)

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Dir = spec.WorkDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %w: %s", strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeComposeOverride(projectID string, services []string) (string, error) {
	f, err := os.CreateTemp("", "last-deploy-compose-*.yml")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	var sb strings.Builder
	sb.WriteString("services:\n")
	for _, svc := range services {
		sb.WriteString(fmt.Sprintf("  %s:\n    labels:\n      %s: %q\n", svc, ProjectIDLabelKey, projectID))
	}

	if _, err := f.WriteString(sb.String()); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// normalizeComposeFile strips any repo path prefix from the compose file path.
// This handles cases where the DB contains paths like "data/repos/<id>/docker-compose.yml"
// instead of just "docker-compose.yml".
func normalizeComposeFile(composeFile, projectID string) string {
	// Convert to forward slashes for consistent matching
	normalized := strings.ReplaceAll(composeFile, "\\", "/")

	// Check if path contains the projectID (indicates a repo path prefix)
	if idx := strings.Index(normalized, projectID+"/"); idx != -1 {
		// Strip everything up to and including the projectID directory
		return normalized[idx+len(projectID)+1:]
	}

	return composeFile
}
