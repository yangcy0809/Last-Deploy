package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"last-deploy/internal/config"
	"last-deploy/internal/engine"
	"last-deploy/internal/store"
	"last-deploy/internal/workspace"
)

type Worker struct {
	st    *store.Store
	queue *Queue
	cfg   config.Config
}

func NewWorker(st *store.Store, q *Queue, cfg config.Config) *Worker {
	return &Worker{st: st, queue: q, cfg: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case jobID := <-w.queue.C():
			w.runJob(ctx, jobID)
		}
	}
}

func (w *Worker) runJob(ctx context.Context, jobID string) {
	job, err := w.st.GetJob(ctx, jobID)
	if err != nil {
		return
	}
	if job.Status != store.JobStatusQueued {
		return
	}

	_ = w.st.SetJobRunning(ctx, jobID, "init")
	_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("%s job started\n", time.Now().Format(time.RFC3339)))

	project, err := w.st.GetProject(ctx, job.ProjectID)
	if err != nil {
		w.fail(ctx, jobID, fmt.Errorf("load project: %w", err))
		return
	}

	switch job.Type {
	case store.JobTypeDeploy:
		err = w.deploy(ctx, project, jobID)
	case store.JobTypeStart:
		err = w.start(ctx, project, jobID)
	case store.JobTypeStop:
		err = w.stop(ctx, project, jobID)
	case store.JobTypePause:
		err = w.pause(ctx, project, jobID)
	case store.JobTypeUnpause:
		err = w.unpause(ctx, project, jobID)
	case store.JobTypeDelete:
		err = w.delete(ctx, project, jobID)
	default:
		err = fmt.Errorf("unknown job type: %q", job.Type)
	}
	if err != nil {
		w.fail(ctx, jobID, err)
		return
	}

	_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("%s job finished\n", time.Now().Format(time.RFC3339)))
	_ = w.st.SetJobSucceeded(ctx, jobID)
}

func (w *Worker) fail(ctx context.Context, jobID string, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("%s error: %v\n", time.Now().Format(time.RFC3339), err))
	_ = w.st.SetJobFailed(ctx, jobID, err.Error())
}

func (w *Worker) deploy(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "set_project_status")
	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusDeploying)

	if err := w.cloneProject(ctx, project, jobID); err != nil {
		_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
		return err
	}

	// 写入配置文件
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
		return fmt.Errorf("work dir: %w", err)
	}

	// 写入 Dockerfile（如果有内容）
	if project.DockerfileContent != "" && project.DockerfilePath != "" {
		fullPath := filepath.Join(workDir, project.DockerfilePath)
		if err := os.WriteFile(fullPath, []byte(project.DockerfileContent), 0644); err != nil {
			_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
			return fmt.Errorf("write dockerfile: %w", err)
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("wrote Dockerfile to %s\n", project.DockerfilePath))
	}

	// 写入 docker-compose.yml（如果有内容）
	if project.ComposeContent != "" && project.ComposeFile != "" {
		fullPath := filepath.Join(workDir, project.ComposeFile)
		if err := os.WriteFile(fullPath, []byte(project.ComposeContent), 0644); err != nil {
			_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
			return fmt.Errorf("write compose: %w", err)
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("wrote docker-compose to %s\n", project.ComposeFile))
	}

	switch engine.ResolveDeployType(project.DeployType, project.ComposeFile) {
	case engine.DeployTypeCompose:
		if err := w.composeUp(ctx, project, jobID); err != nil {
			_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
			return err
		}
	default:
		if err := w.dockerfileDeploy(ctx, project, jobID); err != nil {
			_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusFailed)
			return err
		}
	}

	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusRunning)
	return nil
}

func (w *Worker) start(ctx context.Context, project store.Project, jobID string) error {
	switch engine.ResolveDeployType(project.DeployType, project.ComposeFile) {
	case engine.DeployTypeCompose:
		if err := w.cloneProject(ctx, project, jobID); err != nil {
			return err
		}
		if err := w.composeUp(ctx, project, jobID); err != nil {
			return err
		}
	default:
		dk, err := engine.NewDocker()
		if err != nil {
			return err
		}
		defer dk.Close()

		_ = w.st.SetJobStep(ctx, jobID, "docker_start")
		n, err := dk.StartProjectContainers(ctx, project.ID)
		if err != nil {
			return err
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("started %d container(s)\n", n))
	}

	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusRunning)
	return nil
}

func (w *Worker) stop(ctx context.Context, project store.Project, jobID string) error {
	switch engine.ResolveDeployType(project.DeployType, project.ComposeFile) {
	case engine.DeployTypeCompose:
		if err := w.cloneProject(ctx, project, jobID); err != nil {
			return err
		}
		if err := w.composeStop(ctx, project, jobID); err != nil {
			return err
		}
	default:
		dk, err := engine.NewDocker()
		if err != nil {
			return err
		}
		defer dk.Close()

		_ = w.st.SetJobStep(ctx, jobID, "docker_stop")
		n, err := dk.StopProjectContainers(ctx, project.ID, 10*time.Second)
		if err != nil {
			return err
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("stopped %d container(s)\n", n))
	}

	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusStopped)
	return nil
}

func (w *Worker) pause(ctx context.Context, project store.Project, jobID string) error {
	switch engine.ResolveDeployType(project.DeployType, project.ComposeFile) {
	case engine.DeployTypeCompose:
		if err := w.cloneProject(ctx, project, jobID); err != nil {
			return err
		}
		if err := w.composePause(ctx, project, jobID); err != nil {
			return err
		}
	default:
		dk, err := engine.NewDocker()
		if err != nil {
			return err
		}
		defer dk.Close()

		_ = w.st.SetJobStep(ctx, jobID, "docker_pause")
		n, err := dk.PauseProjectContainers(ctx, project.ID)
		if err != nil {
			return err
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("paused %d container(s)\n", n))
	}

	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusPaused)
	return nil
}

func (w *Worker) unpause(ctx context.Context, project store.Project, jobID string) error {
	switch engine.ResolveDeployType(project.DeployType, project.ComposeFile) {
	case engine.DeployTypeCompose:
		if err := w.cloneProject(ctx, project, jobID); err != nil {
			return err
		}
		if err := w.composeUnpause(ctx, project, jobID); err != nil {
			return err
		}
	default:
		dk, err := engine.NewDocker()
		if err != nil {
			return err
		}
		defer dk.Close()

		_ = w.st.SetJobStep(ctx, jobID, "docker_unpause")
		n, err := dk.UnpauseProjectContainers(ctx, project.ID)
		if err != nil {
			return err
		}
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("unpaused %d container(s)\n", n))
	}

	_ = w.st.SetProjectStatus(ctx, project.ID, store.ProjectStatusRunning)
	return nil
}

func (w *Worker) delete(ctx context.Context, project store.Project, jobID string) error {
	dk, err := engine.NewDocker()
	if err != nil {
		return err
	}
	defer dk.Close()

	// 统一清理 Docker 资源（容器、网络、镜像）
	_ = w.st.SetJobStep(ctx, jobID, "docker_cleanup")
	_ = dk.RemoveProjectContainers(ctx, project.ID)
	_ = dk.RemoveProjectNetworks(ctx, project.ID)
	_ = dk.RemoveProjectImage(ctx, project.ID)

	_ = w.st.SetJobStep(ctx, jobID, "remove_repo")
	_ = os.RemoveAll(workspace.RepoDir(w.cfg, project.ID))

	_ = w.st.SetJobStep(ctx, jobID, "mark_deleted")
	return w.st.MarkProjectDeleted(ctx, project.ID)
}

func (w *Worker) cloneProject(ctx context.Context, project store.Project, jobID string) error {
	repoDir := workspace.RepoDir(w.cfg, project.ID)
	_ = w.st.SetJobStep(ctx, jobID, "sync_repo")

	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("fetching %s\n", project.GitURL))
	} else {
		_ = w.st.AppendJobLog(ctx, jobID, fmt.Sprintf("cloning %s\n", project.GitURL))
	}
	return engine.CloneRepo(ctx, project.GitURL, project.GitRef, repoDir)
}

func (w *Worker) dockerfileDeploy(ctx context.Context, project store.Project, jobID string) error {
	dk, err := engine.NewDocker()
	if err != nil {
		return err
	}
	defer dk.Close()

	_ = w.st.SetJobStep(ctx, jobID, "docker_cleanup")
	if err := dk.RemoveProjectContainers(ctx, project.ID); err != nil {
		return err
	}

	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	_ = w.st.SetJobStep(ctx, jobID, "docker_build")
	if err := dk.BuildProjectImage(ctx, project.ID, workDir, project.DockerfilePath); err != nil {
		return err
	}

	_ = w.st.SetJobStep(ctx, jobID, "docker_run")
	return dk.RunProjectContainer(ctx, project.ID, project.HostPort, project.ContainerPort)
}

func (w *Worker) composeUp(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "compose_up")
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	hostWorkDir, _ := workspace.HostWorkDir(w.cfg, project)
	return engine.ComposeUp(ctx, engine.ComposeSpec{
		ProjectID:      project.ID,
		WorkDir:        workDir,
		HostWorkDir:    hostWorkDir,
		ComposeFile:    project.ComposeFile,
		ComposeService: project.ComposeService,
	})
}

func (w *Worker) composeStop(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "compose_stop")
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	hostWorkDir, _ := workspace.HostWorkDir(w.cfg, project)
	return engine.ComposeStop(ctx, engine.ComposeSpec{
		ProjectID:      project.ID,
		WorkDir:        workDir,
		HostWorkDir:    hostWorkDir,
		ComposeFile:    project.ComposeFile,
		ComposeService: project.ComposeService,
	})
}

func (w *Worker) composePause(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "compose_pause")
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	hostWorkDir, _ := workspace.HostWorkDir(w.cfg, project)
	return engine.ComposePause(ctx, engine.ComposeSpec{
		ProjectID:      project.ID,
		WorkDir:        workDir,
		HostWorkDir:    hostWorkDir,
		ComposeFile:    project.ComposeFile,
		ComposeService: project.ComposeService,
	})
}

func (w *Worker) composeUnpause(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "compose_unpause")
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	hostWorkDir, _ := workspace.HostWorkDir(w.cfg, project)
	return engine.ComposeUnpause(ctx, engine.ComposeSpec{
		ProjectID:      project.ID,
		WorkDir:        workDir,
		HostWorkDir:    hostWorkDir,
		ComposeFile:    project.ComposeFile,
		ComposeService: project.ComposeService,
	})
}

func (w *Worker) composeDown(ctx context.Context, project store.Project, jobID string) error {
	_ = w.st.SetJobStep(ctx, jobID, "compose_down")
	workDir, err := workspace.WorkDir(w.cfg, project)
	if err != nil {
		return fmt.Errorf("work dir: %w", err)
	}
	hostWorkDir, _ := workspace.HostWorkDir(w.cfg, project)
	return engine.ComposeDown(ctx, engine.ComposeSpec{
		ProjectID:      project.ID,
		WorkDir:        workDir,
		HostWorkDir:    hostWorkDir,
		ComposeFile:    project.ComposeFile,
		ComposeService: project.ComposeService,
	})
}
