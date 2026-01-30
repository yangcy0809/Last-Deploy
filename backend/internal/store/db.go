package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "embed"
	_ "github.com/mattn/go-sqlite3"
)

var ErrNotFound = errors.New("not found")

const (
	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
)

const (
	JobTypeDeploy  = "deploy"
	JobTypeStart   = "start"
	JobTypeStop    = "stop"
	JobTypePause   = "pause"
	JobTypeUnpause = "unpause"
	JobTypeDelete  = "delete"
)

const (
	ProjectStatusUnknown   = "unknown"
	ProjectStatusRunning   = "running"
	ProjectStatusPaused    = "paused"
	ProjectStatusStopped   = "stopped"
	ProjectStatusFailed    = "failed"
	ProjectStatusDeleted   = "deleted"
	ProjectStatusDeploying = "deploying"
)

type Store struct {
	db *sql.DB
}

//go:embed schema.sql
var schemaSQL string

func Open(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.init(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("pragma journal_mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA synchronous = NORMAL;`); err != nil {
		return fmt.Errorf("pragma synchronous: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	// Add dockerfile_content column to projects if missing.
	var dfCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'dockerfile_content'`).Scan(&dfCount)
	if err != nil {
		return fmt.Errorf("check dockerfile_content column: %w", err)
	}
	if dfCount == 0 {
		if _, err := s.db.ExecContext(ctx,
			`ALTER TABLE projects ADD COLUMN dockerfile_content TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add dockerfile_content column: %w", err)
		}
	}

	// Add compose_content column to projects if missing.
	var ccCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'compose_content'`).Scan(&ccCount)
	if err != nil {
		return fmt.Errorf("check compose_content column: %w", err)
	}
	if ccCount == 0 {
		if _, err := s.db.ExecContext(ctx,
			`ALTER TABLE projects ADD COLUMN compose_content TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add compose_content column: %w", err)
		}
	}

	// Migrate old config_content to new columns if config_content column exists.
	var oldCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'config_content'`).Scan(&oldCount)
	if err != nil {
		return fmt.Errorf("check config_content column: %w", err)
	}
	if oldCount > 0 {
		// Migrate compose projects: copy config_content to compose_content
		if _, err := s.db.ExecContext(ctx, `
			UPDATE projects SET compose_content = config_content
			WHERE deploy_type = 'compose' AND config_content != '' AND compose_content = ''`); err != nil {
			return fmt.Errorf("migrate compose config_content: %w", err)
		}
		// Migrate dockerfile projects: copy config_content to dockerfile_content
		if _, err := s.db.ExecContext(ctx, `
			UPDATE projects SET dockerfile_content = config_content
			WHERE deploy_type != 'compose' AND config_content != '' AND dockerfile_content = ''`); err != nil {
			return fmt.Errorf("migrate dockerfile config_content: %w", err)
		}
	}

	// Fix bad compose_file paths that contain repo directory prefix.
	// These paths look like "data/repos/<id>/docker-compose.yml" but should just be "docker-compose.yml".
	rows, err := s.db.QueryContext(ctx, `SELECT id, compose_file FROM projects WHERE compose_file != ''`)
	if err != nil {
		return fmt.Errorf("query compose_file: %w", err)
	}
	defer rows.Close()

	var fixes []struct{ id, newPath string }
	for rows.Next() {
		var id, composePath string
		if err := rows.Scan(&id, &composePath); err != nil {
			return fmt.Errorf("scan compose_file: %w", err)
		}
		// Check if compose_file contains the project ID (indicates a bad path)
		if idx := strings.Index(composePath, id+"/"); idx != -1 {
			newPath := composePath[idx+len(id)+1:]
			fixes = append(fixes, struct{ id, newPath string }{id, newPath})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate compose_file: %w", err)
	}

	for _, fix := range fixes {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE projects SET compose_file = ? WHERE id = ?`, fix.newPath, fix.id); err != nil {
			return fmt.Errorf("fix compose_file for %s: %w", fix.id, err)
		}
	}

	return nil
}

type Project struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	GitURL            string `json:"git_url"`
	GitRef            string `json:"git_ref"`
	RepoSubdir        string `json:"repo_subdir"`
	DeployType        string `json:"deploy_type"`
	ComposeFile       string `json:"compose_file"`
	ComposeService    string `json:"compose_service"`
	DockerfilePath    string `json:"dockerfile_path"`
	DockerfileContent string `json:"dockerfile_content,omitempty"`
	ComposeContent    string `json:"compose_content,omitempty"`
	HostPort          int    `json:"host_port"`
	ContainerPort     int    `json:"container_port"`
	LastStatus        string `json:"last_status"`
	LastStatusAt      *int64 `json:"last_status_at,omitempty"`
	DeletedAt         *int64 `json:"deleted_at,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

type Job struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CurrentStep string `json:"current_step"`
	Log         string `json:"log"`
	Error       string `json:"error"`
	RequestedAt int64  `json:"requested_at"`
	StartedAt   *int64 `json:"started_at,omitempty"`
	FinishedAt  *int64 `json:"finished_at,omitempty"`
}

type ProjectDraft struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	GitURL            string   `json:"git_url"`
	DeployType        string   `json:"deploy_type"`
	DockerfilePath    string   `json:"dockerfile_path"`
	DockerfileContent string   `json:"dockerfile_content"`
	ComposePath       string   `json:"compose_path"`
	ComposeContent    string   `json:"compose_content"`
	Services          []string `json:"services"`
	RepoDir           string   `json:"repo_dir"`
	CreatedAt         int64    `json:"created_at"`
	ExpiresAt         int64    `json:"expires_at"`
}

func (s *Store) CreateProjectDraft(ctx context.Context, d ProjectDraft) (ProjectDraft, error) {
	if d.ID == "" {
		return ProjectDraft{}, fmt.Errorf("draft id is required")
	}
	if d.Name == "" {
		return ProjectDraft{}, fmt.Errorf("draft name is required")
	}
	if d.GitURL == "" {
		return ProjectDraft{}, fmt.Errorf("draft git_url is required")
	}
	if d.DeployType == "" {
		return ProjectDraft{}, fmt.Errorf("draft deploy_type is required")
	}
	if d.RepoDir == "" {
		return ProjectDraft{}, fmt.Errorf("draft repo_dir is required")
	}
	if d.ExpiresAt == 0 {
		return ProjectDraft{}, fmt.Errorf("draft expires_at is required")
	}

	now := time.Now().Unix()
	if d.CreatedAt == 0 {
		d.CreatedAt = now
	}

	servicesJSON, err := json.Marshal(d.Services)
	if err != nil {
		return ProjectDraft{}, fmt.Errorf("marshal services: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO project_drafts (
		  id, name, git_url, deploy_type, dockerfile_path, dockerfile_content,
		  compose_path, compose_content, services_json, repo_dir, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.GitURL, d.DeployType, d.DockerfilePath, d.DockerfileContent,
		d.ComposePath, d.ComposeContent, string(servicesJSON), d.RepoDir, d.CreatedAt, d.ExpiresAt)
	if err != nil {
		return ProjectDraft{}, err
	}
	return d, nil
}

func (s *Store) ListExpiredProjectDrafts(ctx context.Context, now int64) ([]ProjectDraft, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo_dir
		FROM project_drafts
		WHERE expires_at <= ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProjectDraft
	for rows.Next() {
		var d ProjectDraft
		if err := rows.Scan(&d.ID, &d.RepoDir); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetProjectDraft(ctx context.Context, id string) (ProjectDraft, error) {
	if id == "" {
		return ProjectDraft{}, fmt.Errorf("draft id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, git_url, deploy_type, dockerfile_path, dockerfile_content,
		       compose_path, compose_content, services_json, repo_dir, created_at, expires_at
		FROM project_drafts
		WHERE id = ?`, id)

	var d ProjectDraft
	var servicesJSON string
	err := row.Scan(&d.ID, &d.Name, &d.GitURL, &d.DeployType, &d.DockerfilePath, &d.DockerfileContent,
		&d.ComposePath, &d.ComposeContent, &servicesJSON, &d.RepoDir, &d.CreatedAt, &d.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectDraft{}, ErrNotFound
		}
		return ProjectDraft{}, err
	}
	if err := json.Unmarshal([]byte(servicesJSON), &d.Services); err != nil {
		return ProjectDraft{}, fmt.Errorf("unmarshal services: %w", err)
	}
	return d, nil
}

func (s *Store) DeleteProjectDraft(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("draft id is required")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM project_drafts WHERE id = ?`, id)
	return err
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, git_url, git_ref, repo_subdir, deploy_type, compose_file, compose_service,
		       dockerfile_path, dockerfile_content, compose_content, host_port, container_port, last_status, last_status_at, deleted_at,
		       created_at, updated_at
		FROM projects
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, git_url, git_ref, repo_subdir, deploy_type, compose_file, compose_service,
		       dockerfile_path, dockerfile_content, compose_content, host_port, container_port, last_status, last_status_at, deleted_at,
		       created_at, updated_at
		FROM projects
		WHERE id = ? AND deleted_at IS NULL`, id)
	p, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, err
	}
	return p, nil
}

func (s *Store) CreateProject(ctx context.Context, p Project) (Project, error) {
	now := time.Now().Unix()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = now
	}
	if p.LastStatus == "" {
		p.LastStatus = ProjectStatusUnknown
	}
	if p.DeployType == "" {
		p.DeployType = "auto"
	}
	if p.DockerfilePath == "" {
		p.DockerfilePath = "Dockerfile"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects (
		  id, name, git_url, git_ref, repo_subdir, deploy_type, compose_file, compose_service,
		  dockerfile_path, dockerfile_content, compose_content, host_port, container_port, last_status, last_status_at, deleted_at,
		  created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.GitURL, p.GitRef, p.RepoSubdir, p.DeployType, p.ComposeFile, p.ComposeService,
		p.DockerfilePath, p.DockerfileContent, p.ComposeContent, p.HostPort, p.ContainerPort, p.LastStatus, nil, nil, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

func (s *Store) SetProjectStatus(ctx context.Context, id, status string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET last_status = ?, last_status_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`, status, now, now, id)
	return err
}

func (s *Store) MarkProjectDeleted(ctx context.Context, id string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET deleted_at = ?, updated_at = ?, last_status = ?, last_status_at = ?
		WHERE id = ? AND deleted_at IS NULL`, now, now, ProjectStatusDeleted, now, id)
	return err
}

func (s *Store) UpdateProjectConfig(ctx context.Context, id, dockerfileContent, composeContent string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET dockerfile_content = ?, compose_content = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`, dockerfileContent, composeContent, now, id)
	return err
}

func (s *Store) UpdateProjectConfigWithPorts(ctx context.Context, id, dockerfileContent, composeContent string, hostPort, containerPort int) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET dockerfile_content = ?, compose_content = ?, host_port = ?, container_port = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`, dockerfileContent, composeContent, hostPort, containerPort, now, id)
	return err
}

func (s *Store) CreateJob(ctx context.Context, j Job) (Job, error) {
	now := time.Now().Unix()
	if j.RequestedAt == 0 {
		j.RequestedAt = now
	}
	if j.Status == "" {
		j.Status = JobStatusQueued
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (
		  id, project_id, type, status, current_step, log, error,
		  requested_at, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.ProjectID, j.Type, j.Status, j.CurrentStep, j.Log, j.Error, j.RequestedAt, nil, nil)
	if err != nil {
		return Job{}, err
	}
	return j, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, type, status, current_step, log, error,
		       requested_at, started_at, finished_at
		FROM jobs
		WHERE id = ?`, id)

	j, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	return j, nil
}

func (s *Store) ListJobsByStatus(ctx context.Context, status string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, type, status, current_step, log, error,
		       requested_at, started_at, finished_at
		FROM jobs
		WHERE status = ?
		ORDER BY requested_at ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) GetLatestJobByProject(ctx context.Context, projectID string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, type, status, current_step, log, error,
		       requested_at, started_at, finished_at
		FROM jobs
		WHERE project_id = ?
		ORDER BY requested_at DESC
		LIMIT 1`, projectID)
	j, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	return j, nil
}

func (s *Store) SetJobRunning(ctx context.Context, id, step string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?, current_step = ?, started_at = ?
		WHERE id = ?`, JobStatusRunning, step, now, id)
	return err
}

func (s *Store) SetJobStep(ctx context.Context, id, step string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET current_step = ?
		WHERE id = ?`, step, id)
	return err
}

func (s *Store) AppendJobLog(ctx context.Context, id, line string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET log = log || ?
		WHERE id = ?`, line, id)
	return err
}

func (s *Store) SetJobFailed(ctx context.Context, id string, msg string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?, error = ?, finished_at = ?
		WHERE id = ?`, JobStatusFailed, msg, now, id)
	return err
}

func (s *Store) SetJobSucceeded(ctx context.Context, id string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?, finished_at = ?
		WHERE id = ?`, JobStatusSucceeded, now, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(s scanner) (Project, error) {
	var lastStatusAt sql.NullInt64
	var deletedAt sql.NullInt64
	var p Project
	err := s.Scan(
		&p.ID, &p.Name, &p.GitURL, &p.GitRef, &p.RepoSubdir, &p.DeployType, &p.ComposeFile, &p.ComposeService,
		&p.DockerfilePath, &p.DockerfileContent, &p.ComposeContent, &p.HostPort, &p.ContainerPort, &p.LastStatus, &lastStatusAt, &deletedAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return Project{}, err
	}
	if lastStatusAt.Valid {
		v := lastStatusAt.Int64
		p.LastStatusAt = &v
	}
	if deletedAt.Valid {
		v := deletedAt.Int64
		p.DeletedAt = &v
	}
	return p, nil
}

func scanJob(s scanner) (Job, error) {
	var startedAt sql.NullInt64
	var finishedAt sql.NullInt64
	var j Job
	err := s.Scan(
		&j.ID, &j.ProjectID, &j.Type, &j.Status, &j.CurrentStep, &j.Log, &j.Error,
		&j.RequestedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return Job{}, err
	}
	if startedAt.Valid {
		v := startedAt.Int64
		j.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Int64
		j.FinishedAt = &v
	}
	return j, nil
}
