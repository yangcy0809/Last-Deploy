package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"last-deploy/internal/detector"
	"last-deploy/internal/engine"
	"last-deploy/internal/store"
)

var composeServiceRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

type createProjectRequest struct {
	Name           string `json:"name"`
	GitURL         string `json:"git_url"`
	GitRef         string `json:"git_ref"`
	RepoSubdir     string `json:"repo_subdir"`
	DeployType     string `json:"deploy_type"`
	ComposeFile    string `json:"compose_file"`
	ComposeService string `json:"compose_service"`
	DockerfilePath string `json:"dockerfile_path"`
	HostPort       int    `json:"host_port"`
	ContainerPort  int    `json:"container_port"`
	Deploy         bool   `json:"deploy"`
}

func (s *Server) listProjects(c *gin.Context) {
	projects, err := s.st.ListProjects(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (s *Server) createProject(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if strings.TrimSpace(req.GitURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "git_url is required"})
		return
	}
	if req.HostPort <= 0 || req.HostPort > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid host_port"})
		return
	}
	if req.ContainerPort <= 0 || req.ContainerPort > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid container_port"})
		return
	}

	deployType := strings.ToLower(strings.TrimSpace(req.DeployType))
	if deployType == "" {
		deployType = "auto"
	}
	switch deployType {
	case "auto", "dockerfile", "compose":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deploy_type"})
		return
	}

	composeService := strings.TrimSpace(req.ComposeService)
	if composeService != "" {
		// 验证每个服务名（支持逗号分隔）
		for _, svc := range strings.Split(composeService, ",") {
			svc = strings.TrimSpace(svc)
			if svc != "" && !composeServiceRe.MatchString(svc) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid compose_service: " + svc})
				return
			}
		}
	}

	id, err := newID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().Unix()
	project, err := s.st.CreateProject(c.Request.Context(), store.Project{
		ID:             id,
		Name:           req.Name,
		GitURL:         req.GitURL,
		GitRef:         req.GitRef,
		RepoSubdir:     req.RepoSubdir,
		DeployType:     deployType,
		ComposeFile:    req.ComposeFile,
		ComposeService: composeService,
		DockerfilePath: req.DockerfilePath,
		HostPort:       req.HostPort,
		ContainerPort:  req.ContainerPort,
		LastStatus:     store.ProjectStatusUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !req.Deploy {
		c.JSON(http.StatusCreated, gin.H{"project": project})
		return
	}

	job, err := s.createJob(c.Request.Context(), project.ID, store.JobTypeDeploy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"project": project, "job": job})
}

func (s *Server) getProject(c *gin.Context) {
	id := c.Param("id")
	p, err := s.st.GetProject(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": p})
}

func (s *Server) getProjectLatestJob(c *gin.Context) {
	id := c.Param("id")
	job, err := s.st.GetLatestJobByProject(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (s *Server) deployProject(c *gin.Context)  { s.enqueueJob(c, store.JobTypeDeploy) }
func (s *Server) startProject(c *gin.Context)   { s.enqueueJob(c, store.JobTypeStart) }
func (s *Server) stopProject(c *gin.Context)    { s.enqueueJob(c, store.JobTypeStop) }
func (s *Server) pauseProject(c *gin.Context)   { s.enqueueJob(c, store.JobTypePause) }
func (s *Server) unpauseProject(c *gin.Context) { s.enqueueJob(c, store.JobTypeUnpause) }
func (s *Server) deleteProject(c *gin.Context)  { s.enqueueJob(c, store.JobTypeDelete) }

func (s *Server) enqueueJob(c *gin.Context, jobType string) {
	projectID := c.Param("id")
	if _, err := s.st.GetProject(c.Request.Context(), projectID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	job, err := s.createJob(c.Request.Context(), projectID, jobType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (s *Server) createJob(ctx context.Context, projectID, jobType string) (store.Job, error) {
	id, err := newID()
	if err != nil {
		return store.Job{}, err
	}
	job, err := s.st.CreateJob(ctx, store.Job{
		ID:        id,
		ProjectID: projectID,
		Type:      jobType,
		Status:    store.JobStatusQueued,
	})
	if err != nil {
		return store.Job{}, err
	}
	s.queue.Enqueue(job.ID)
	return job, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type detectProjectRequest struct {
	Name   string `json:"name"`
	GitURL string `json:"git_url"`
}

func (s *Server) detectProject(c *gin.Context) {
	var req detectProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if strings.TrimSpace(req.GitURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "git_url is required"})
		return
	}

	id, err := newID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	repoDir := filepath.Join(os.TempDir(), "last-deploy-drafts", id)
	if err := engine.CloneRepo(c.Request.Context(), req.GitURL, "", repoDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "clone failed: " + err.Error()})
		return
	}

	result, err := detector.Detect(repoDir)
	if err != nil {
		_ = os.RemoveAll(repoDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "detect failed: " + err.Error()})
		return
	}

	now := time.Now().Unix()
	draft := store.ProjectDraft{
		ID:                id,
		Name:              req.Name,
		GitURL:            req.GitURL,
		DeployType:        result.DeployType,
		DockerfilePath:    result.DockerfilePath,
		DockerfileContent: result.DockerfileContent,
		ComposePath:       result.ComposePath,
		ComposeContent:    result.ComposeContent,
		Services:          result.Services,
		RepoDir:           repoDir,
		CreatedAt:         now,
		ExpiresAt:         now + 30*60, // 30 minutes
	}
	if _, err := s.st.CreateProjectDraft(c.Request.Context(), draft); err != nil {
		_ = os.RemoveAll(repoDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"draft_id":           id,
		"deploy_type":        result.DeployType,
		"dockerfile_path":    result.DockerfilePath,
		"dockerfile_content": result.DockerfileContent,
		"compose_path":       result.ComposePath,
		"compose_content":    result.ComposeContent,
		"services":           result.Services,
	})
}

type createProjectFromDraftRequest struct {
	DraftID           string `json:"draft_id"`
	DockerfileContent string `json:"dockerfile_content"`
	ComposeContent    string `json:"compose_content"`
	ComposeService    string `json:"compose_service"`
	GitRef            string `json:"git_ref"`
	RepoSubdir        string `json:"repo_subdir"`
	Deploy            bool   `json:"deploy"`
}

type updateProjectConfigRequest struct {
	DockerfileContent string `json:"dockerfile_content"`
	ComposeContent    string `json:"compose_content"`
}

func (s *Server) updateProjectConfig(c *gin.Context) {
	id := c.Param("id")
	var req updateProjectConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := s.st.GetProject(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据部署类型解析端口
	var hostPort, containerPort int
	if project.DeployType == "compose" && req.ComposeContent != "" {
		hostPort, containerPort = parseComposePort(req.ComposeContent, project.ComposeService)
	} else if req.DockerfileContent != "" {
		containerPort = parseDockerfilePort(req.DockerfileContent)
		hostPort = containerPort
	}

	// 如果解析到了端口，同步更新
	if hostPort > 0 && containerPort > 0 {
		if err := s.st.UpdateProjectConfigWithPorts(c.Request.Context(), id, req.DockerfileContent, req.ComposeContent, hostPort, containerPort); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 没有解析到端口，只更新配置内容
		if err := s.st.UpdateProjectConfig(c.Request.Context(), id, req.DockerfileContent, req.ComposeContent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) createProjectFromDraft(c *gin.Context) {
	var req createProjectFromDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.DraftID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "draft_id is required"})
		return
	}

	draft, err := s.st.GetProjectDraft(c.Request.Context(), req.DraftID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "draft not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取用户提交的内容，如果为空则使用 draft 中的内容
	dockerfileContent := strings.TrimSpace(req.DockerfileContent)
	if dockerfileContent == "" {
		dockerfileContent = draft.DockerfileContent
	}
	composeContent := strings.TrimSpace(req.ComposeContent)
	if composeContent == "" && draft.DeployType != "none" {
		// none 类型不回退到 draft 默认模板，由用户主动填写来决定部署方式
		composeContent = draft.ComposeContent
	}

	// 根据 deploy_type 设置 ComposeFile/DockerfilePath/ComposeService
	var composeFile, dockerfilePath, composeService string
	deployType := draft.DeployType

	// none 时根据用户是否填写了 compose_content 来决定部署类型
	if deployType == "none" {
		if composeContent != "" {
			deployType = "compose"
		} else {
			deployType = "dockerfile"
		}
	}

	if deployType == "compose" {
		composeFile = draft.ComposePath
		if composeFile == "" {
			composeFile = "docker-compose.yml"
		}
		dockerfilePath = ""
		// 使用请求中的 service，否则使用第一个 service
		composeService = strings.TrimSpace(req.ComposeService)
		if composeService == "" && len(draft.Services) > 0 {
			composeService = draft.Services[0]
		}
	} else {
		// dockerfile 类型
		composeFile = ""
		dockerfilePath = draft.DockerfilePath
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}
	}

	// 解析端口信息
	var hostPort, containerPort int
	if deployType == "compose" {
		// 从 compose 内容中解析端口
		hostPort, containerPort = parseComposePort(composeContent, composeService)
	} else {
		// 从 dockerfile 内容中解析 EXPOSE 端口
		containerPort = parseDockerfilePort(dockerfileContent)
		if containerPort > 0 {
			// 使用容器端口作为主机端口（简化处理）
			hostPort = containerPort
		}
	}

	// 如果解析失败，返回错误
	if hostPort <= 0 || containerPort <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法从配置中解析端口信息，请检查 Dockerfile 的 EXPOSE 指令或 docker-compose.yml 的 ports 配置"})
		return
	}

	id, err := newID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now().Unix()
	project, err := s.st.CreateProject(c.Request.Context(), store.Project{
		ID:                id,
		Name:              draft.Name,
		GitURL:            draft.GitURL,
		GitRef:            req.GitRef,
		RepoSubdir:        req.RepoSubdir,
		DeployType:        deployType,
		ComposeFile:       composeFile,
		ComposeService:    composeService,
		DockerfilePath:    dockerfilePath,
		DockerfileContent: dockerfileContent,
		ComposeContent:    composeContent,
		HostPort:          hostPort,
		ContainerPort:     containerPort,
		LastStatus:        store.ProjectStatusUnknown,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除 draft 和临时目录
	_ = os.RemoveAll(draft.RepoDir)
	_ = s.st.DeleteProjectDraft(c.Request.Context(), req.DraftID)

	if !req.Deploy {
		c.JSON(http.StatusCreated, gin.H{"project": project})
		return
	}

	job, err := s.createJob(c.Request.Context(), project.ID, store.JobTypeDeploy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"project": project, "job": job})
}
