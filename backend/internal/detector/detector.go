package detector

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type DetectResult struct {
	DeployType        string   // "compose" | "dockerfile" | "none"
	DockerfilePath    string   // Dockerfile 路径（相对 repoDir）
	DockerfileContent string   // Dockerfile 内容
	ComposePath       string   // compose 文件路径（相对 repoDir）
	ComposeContent    string   // compose 文件内容
	Services          []string // compose 项目的 service 列表
}

const defaultDockerfileTemplate = `FROM alpine:3.20
WORKDIR /app
COPY . .
EXPOSE 8080
CMD ["busybox", "httpd", "-f", "-p", "8080", "-h", "/app"]
`

const defaultComposeTemplate = `version: "3.8"
services:
  app:
    build: .
    ports:
      - "8080:8080"
`

func Detect(repoDir string) (*DetectResult, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repo dir is required")
	}

	var composePath, composeContent string
	var services []string

	composeCandidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
	for _, rel := range composeCandidates {
		content, ok, err := readFileIfExists(filepath.Join(repoDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		if !ok {
			continue
		}
		composePath = rel
		composeContent = content
		services, err = parseComposeServices([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		break
	}

	var dockerfilePath, dockerfileContent string
	dockerfileRel := "Dockerfile"
	content, ok, err := readFileIfExists(filepath.Join(repoDir, filepath.FromSlash(dockerfileRel)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dockerfileRel, err)
	}
	if ok {
		dockerfilePath = dockerfileRel
		dockerfileContent = content
	}

	var deployType string
	if composePath != "" {
		deployType = "compose"
		// compose 类型如果没有 Dockerfile，也提供默认模板
		if dockerfileContent == "" {
			dockerfileContent = defaultDockerfileTemplate
		}
	} else if dockerfilePath != "" {
		deployType = "dockerfile"
		composeContent = defaultComposeTemplate
	} else {
		deployType = "none"
		dockerfileContent = defaultDockerfileTemplate
		composeContent = defaultComposeTemplate
	}

	return &DetectResult{
		DeployType:        deployType,
		DockerfilePath:    dockerfilePath,
		DockerfileContent: dockerfileContent,
		ComposePath:       composePath,
		ComposeContent:    composeContent,
		Services:          services,
	}, nil
}

func readFileIfExists(path string) (content string, ok bool, _ error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return string(b), true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return "", false, err
}

