PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  git_url TEXT NOT NULL,
  git_ref TEXT NOT NULL DEFAULT '',
  repo_subdir TEXT NOT NULL DEFAULT '',
  deploy_type TEXT NOT NULL DEFAULT 'auto',
  compose_file TEXT NOT NULL DEFAULT '',
  compose_service TEXT NOT NULL DEFAULT '',
  dockerfile_path TEXT NOT NULL DEFAULT 'Dockerfile',
  dockerfile_content TEXT NOT NULL DEFAULT '',
  compose_content TEXT NOT NULL DEFAULT '',
  host_port INTEGER NOT NULL DEFAULT 0,
  container_port INTEGER NOT NULL DEFAULT 0,
  last_status TEXT NOT NULL DEFAULT 'unknown',
  last_status_at INTEGER,
  deleted_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_host_port_active ON projects(host_port) WHERE deleted_at IS NULL AND host_port > 0;

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id),
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  current_step TEXT NOT NULL DEFAULT '',
  log TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  requested_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_jobs_project_requested ON jobs(project_id, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

CREATE TABLE IF NOT EXISTS project_drafts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  git_url TEXT NOT NULL,
  deploy_type TEXT NOT NULL,
  dockerfile_path TEXT NOT NULL DEFAULT '',
  dockerfile_content TEXT NOT NULL DEFAULT '',
  compose_path TEXT NOT NULL DEFAULT '',
  compose_content TEXT NOT NULL DEFAULT '',
  services_json TEXT NOT NULL DEFAULT '[]',
  repo_dir TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_drafts_expires_at ON project_drafts(expires_at);
