import {
  DeleteOutlined,
  EditOutlined,
  FileTextOutlined,
  LinkOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PoweroffOutlined,
  RocketOutlined,
} from '@ant-design/icons'
import { Button, Popconfirm, Space, Table, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { Project } from '../api/types'
import { ProjectStatusTag } from './StatusTag'

type Props = {
  projects: Project[]
  loading?: boolean
  onDeploy: (project: Project) => void
  onStart: (project: Project) => void
  onStop: (project: Project) => void
  onPause: (project: Project) => void
  onUnpause: (project: Project) => void
  onDelete: (project: Project) => void
  onViewLog: (project: Project) => void
  onEditConfig: (project: Project) => void
}

function normalizeStatus(status?: string): string {
  return (status ?? 'unknown').trim().toLowerCase()
}

function parseComposePorts(configContent: string, serviceName: string): string {
  if (!configContent || !serviceName) return '—'
  try {
    const lines = configContent.split('\n')
    let inService = false
    let inPorts = false
    const ports: string[] = []

    for (const line of lines) {
      const trimmed = line.trim()
      if (trimmed === `${serviceName}:`) {
        inService = true
        continue
      }
      if (inService && trimmed === 'ports:') {
        inPorts = true
        continue
      }
      if (inPorts && trimmed.startsWith('-')) {
        const port = trimmed.replace(/^-\s*["']?/, '').replace(/["']$/, '')
        ports.push(port)
      }
      if (inPorts && !trimmed.startsWith('-') && trimmed && !trimmed.startsWith('#')) {
        break
      }
    }
    return ports.length > 0 ? ports.join(', ') : '—'
  } catch {
    return '—'
  }
}

function extractHostPort(configContent: string, serviceName: string): number | null {
  if (!configContent || !serviceName) return null
  try {
    const lines = configContent.split('\n')
    let inService = false
    let inPorts = false

    for (const line of lines) {
      const trimmed = line.trim()
      if (trimmed === `${serviceName}:`) {
        inService = true
        continue
      }
      if (inService && trimmed === 'ports:') {
        inPorts = true
        continue
      }
      if (inPorts && trimmed.startsWith('-')) {
        const port = trimmed.replace(/^-\s*["']?/, '').replace(/["']$/, '')
        const match = port.match(/^(\d+):/)
        if (match) return parseInt(match[1], 10)
      }
      if (inPorts && !trimmed.startsWith('-') && trimmed && !trimmed.startsWith('#')) {
        break
      }
    }
    return null
  } catch {
    return null
  }
}

export default function ProjectTable({
  projects,
  loading,
  onDeploy,
  onStart,
  onStop,
  onPause,
  onUnpause,
  onDelete,
  onViewLog,
  onEditConfig,
}: Props) {
  const columns: ColumnsType<Project> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 220,
      ellipsis: true,
    },
    {
      title: 'Git URL',
      dataIndex: 'git_url',
      key: 'git_url',
      render: (gitURL: string) =>
        gitURL ? (
          <Typography.Link href={gitURL} target="_blank">
            {gitURL}
          </Typography.Link>
        ) : (
          '—'
        ),
    },
    {
      title: '端口',
      key: 'ports',
      width: 160,
      render: (_: unknown, p: Project) => {
        if (p.deploy_type === 'compose' && p.host_port === 0) {
          return parseComposePorts(p.compose_content, p.compose_service)
        }
        return `${p.host_port} → ${p.container_port}`
      },
    },
    {
      title: '访问链接',
      key: 'url',
      width: 140,
      render: (_: unknown, p: Project) => {
        const status = normalizeStatus(p.last_status)
        if (status !== 'running') return '—'

        let hostPort = p.host_port
        if (p.deploy_type === 'compose' && p.host_port === 0) {
          const extracted = extractHostPort(p.compose_content, p.compose_service)
          if (!extracted) return '—'
          hostPort = extracted
        }

        const url = `http://localhost:${hostPort}`
        return (
          <Typography.Link href={url} target="_blank">
            <LinkOutlined /> 访问
          </Typography.Link>
        )
      },
    },
    {
      title: '状态',
      dataIndex: 'last_status',
      key: 'last_status',
      width: 120,
      render: (status: Project['last_status']) => <ProjectStatusTag status={status} />,
    },
    {
      title: '操作',
      key: 'actions',
      width: 400,
      render: (_: unknown, p: Project) => {
        const status = normalizeStatus(p.last_status)
        const isBusy = status === 'deploying'
        const isRunning = status === 'running'
        const isPaused = status === 'paused'
        const isStopped = status === 'stopped'

        return (
          <Space size="small" wrap>
            <Button
              icon={<RocketOutlined />}
              onClick={() => onDeploy(p)}
              disabled={isBusy}
            >
              部署
            </Button>
            <Button
              icon={<EditOutlined />}
              onClick={() => onEditConfig(p)}
            >
              编辑
            </Button>
            <Button
              icon={<FileTextOutlined />}
              onClick={() => onViewLog(p)}
            >
              日志
            </Button>
            <Button
              icon={<PlayCircleOutlined />}
              onClick={() => onStart(p)}
              disabled={isBusy || isRunning}
            >
              启动
            </Button>
            <Button
              icon={<PoweroffOutlined />}
              onClick={() => onStop(p)}
              disabled={isBusy || isStopped}
            >
              停止
            </Button>
            {isPaused ? (
              <Button
                icon={<PlayCircleOutlined />}
                onClick={() => onUnpause(p)}
                disabled={isBusy}
              >
                恢复
              </Button>
            ) : (
              <Button
                icon={<PauseCircleOutlined />}
                onClick={() => onPause(p)}
                disabled={isBusy || !isRunning}
              >
                暂停
              </Button>
            )}
            <Popconfirm
              title="删除项目？"
              description="会触发删除任务并移除容器/资源。"
              okText="删除"
              cancelText="取消"
              onConfirm={() => onDelete(p)}
              disabled={isBusy}
            >
              <Button danger icon={<DeleteOutlined />} disabled={isBusy}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  return (
    <Table<Project>
      rowKey={(p) => p.id}
      columns={columns}
      dataSource={projects}
      loading={loading}
      size="middle"
      pagination={{ pageSize: 20, showSizeChanger: true }}
      scroll={{ x: true }}
    />
  )
}

