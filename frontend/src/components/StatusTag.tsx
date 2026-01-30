import { Tag } from 'antd'
import type { JobStatus, ProjectStatus } from '../api/types'

function normalizeStatus(status?: string): string {
  return (status ?? 'unknown').trim().toLowerCase()
}

export function ProjectStatusTag({ status }: { status?: ProjectStatus }) {
  const s = normalizeStatus(status)
  switch (s) {
    case 'running':
      return <Tag color="#10b981" style={{ borderColor: '#10b981' }}>运行中</Tag>
    case 'paused':
      return <Tag color="#f59e0b" style={{ borderColor: '#f59e0b' }}>已暂停</Tag>
    case 'stopped':
      return <Tag color="#6b7280" style={{ borderColor: '#6b7280' }}>已停止</Tag>
    case 'failed':
      return <Tag color="#ef4444" style={{ borderColor: '#ef4444' }}>失败</Tag>
    case 'deploying':
      return <Tag color="#8b5cf6" style={{ borderColor: '#8b5cf6' }}>部署中</Tag>
    case 'deleted':
      return <Tag color="#4b5563" style={{ borderColor: '#4b5563' }}>已删除</Tag>
    case 'unknown':
    default:
      return <Tag color="#6b7280" style={{ borderColor: '#6b7280' }}>未知</Tag>
  }
}

export function JobStatusTag({ status }: { status?: JobStatus }) {
  const s = normalizeStatus(status)
  switch (s) {
    case 'queued':
      return <Tag color="#6b7280" style={{ borderColor: '#6b7280' }}>排队中</Tag>
    case 'running':
      return <Tag color="#06b6d4" style={{ borderColor: '#06b6d4' }}>执行中</Tag>
    case 'succeeded':
      return <Tag color="#10b981" style={{ borderColor: '#10b981' }}>成功</Tag>
    case 'failed':
      return <Tag color="#ef4444" style={{ borderColor: '#ef4444' }}>失败</Tag>
    default:
      return <Tag color="#6b7280" style={{ borderColor: '#6b7280' }}>{status ?? 'unknown'}</Tag>
  }
}

