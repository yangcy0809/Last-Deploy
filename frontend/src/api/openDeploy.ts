import { request } from './client'
import type {
  CreateProjectFromDraftRequest,
  CreateProjectRequest,
  DetectProjectRequest,
  DetectProjectResponse,
  Job,
  Project,
} from './types'

export function health(): Promise<{ ok: boolean }> {
  return request('/health')
}

export function listProjects(): Promise<{ projects: Project[] }> {
  return request('/projects')
}

export function createProject(
  body: CreateProjectRequest,
): Promise<{ project: Project; job?: Job }> {
  return request('/projects', { method: 'POST', body: JSON.stringify(body) })
}

export function detectProject(
  body: DetectProjectRequest,
): Promise<DetectProjectResponse> {
  return request('/projects/detect', { method: 'POST', body: JSON.stringify(body) })
}

export function createProjectFromDraft(
  body: CreateProjectFromDraftRequest,
): Promise<{ project: Project; job?: Job }> {
  return request('/projects/from-draft', { method: 'POST', body: JSON.stringify(body) })
}

export function deployProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/deploy`, { method: 'POST' })
}

export function startProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/start`, { method: 'POST' })
}

export function stopProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/stop`, { method: 'POST' })
}

export function pauseProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/pause`, { method: 'POST' })
}

export function unpauseProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/unpause`, { method: 'POST' })
}

export function deleteProject(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function getJob(id: string): Promise<{ job: Job }> {
  return request(`/jobs/${encodeURIComponent(id)}`)
}

export function getProjectLatestJob(id: string): Promise<{ job: Job }> {
  return request(`/projects/${encodeURIComponent(id)}/jobs/latest`)
}

export function updateProjectConfig(
  id: string,
  dockerfileContent: string,
  composeContent: string,
): Promise<{ ok: boolean }> {
  return request(`/projects/${encodeURIComponent(id)}/config`, {
    method: 'PUT',
    body: JSON.stringify({
      dockerfile_content: dockerfileContent,
      compose_content: composeContent,
    }),
  })
}

