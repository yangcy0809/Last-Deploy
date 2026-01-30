export type ProjectStatus =
  | 'unknown'
  | 'running'
  | 'paused'
  | 'stopped'
  | 'failed'
  | 'deleted'
  | 'deploying'
  | (string & {})

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | (string & {})

export type JobType = 'deploy' | 'start' | 'stop' | 'pause' | 'unpause' | 'delete' | (string & {})

export type DeployType = 'auto' | 'dockerfile' | 'compose'

export type UnixSeconds = number

export interface Project {
  id: string
  name: string
  git_url: string
  git_ref: string
  repo_subdir: string
  deploy_type: string
  compose_file: string
  compose_service: string
  dockerfile_path: string
  dockerfile_content: string
  compose_content: string
  host_port: number
  container_port: number
  last_status: ProjectStatus
  last_status_at?: UnixSeconds | null
  deleted_at?: UnixSeconds | null
  created_at: UnixSeconds
  updated_at: UnixSeconds
}

export interface Job {
  id: string
  project_id: string
  type: JobType
  status: JobStatus
  current_step: string
  log: string
  error: string
  requested_at: UnixSeconds
  started_at?: UnixSeconds | null
  finished_at?: UnixSeconds | null
}

export interface CreateProjectRequest {
  name: string
  git_url: string
  host_port: number
  container_port: number
  deploy?: boolean

  git_ref?: string
  repo_subdir?: string
  deploy_type?: DeployType
  compose_file?: string
  compose_service?: string
  dockerfile_path?: string
}

export interface DetectProjectRequest {
  name: string
  git_url: string
}

export interface DetectProjectResponse {
  draft_id: string
  deploy_type: 'compose' | 'dockerfile' | 'none'
  dockerfile_path: string
  dockerfile_content: string
  compose_path: string
  compose_content: string
  services: string[]
}

export interface CreateProjectFromDraftRequest {
  draft_id: string
  dockerfile_content: string
  compose_content: string
  compose_service?: string
  git_ref?: string
  repo_subdir?: string
  deploy?: boolean
}

