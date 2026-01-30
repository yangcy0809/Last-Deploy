package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"last-deploy/internal/config"
	"last-deploy/internal/store"
)

func EnsureDataDirs(cfg config.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.ReposDir(), 0o755); err != nil {
		return err
	}
	return nil
}

func RepoDir(cfg config.Config, projectID string) string {
	return filepath.Join(cfg.ReposDir(), projectID)
}

func SafeJoin(base, rel string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("base is required")
	}

	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return filepath.Clean(base), nil
	}

	normalized := strings.ReplaceAll(rel, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid path: contains '..'")
		}
	}

	relOS := filepath.FromSlash(normalized)
	if filepath.IsAbs(relOS) || filepath.VolumeName(relOS) != "" {
		return "", fmt.Errorf("invalid path: must be relative")
	}

	joined := filepath.Join(base, relOS)

	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	relToBase, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", err
	}
	if relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path: escapes base dir")
	}
	return joined, nil
}

func WorkDir(cfg config.Config, project store.Project) (string, error) {
	repoDir := RepoDir(cfg, project.ID)
	if project.RepoSubdir == "" {
		return repoDir, nil
	}
	return SafeJoin(repoDir, project.RepoSubdir)
}

func HostWorkDir(cfg config.Config, project store.Project) (string, error) {
	if cfg.HostDataDir == "" {
		return "", nil
	}
	hostRepoDir := filepath.Join(cfg.HostDataDir, "repos", project.ID)
	if project.RepoSubdir == "" {
		return hostRepoDir, nil
	}
	return SafeJoin(hostRepoDir, project.RepoSubdir)
}
