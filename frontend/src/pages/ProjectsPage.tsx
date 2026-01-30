import { PlusOutlined, ReloadOutlined, RocketOutlined } from '@ant-design/icons'
import { Alert, Button, Layout, Space, message } from 'antd'
import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import * as api from '../api/openDeploy'
import type { Job, Project } from '../api/types'
import ConfigEditorModal from '../components/ConfigEditorModal'
import JobDrawer from '../components/JobDrawer'
import NewProjectWizardModal from '../components/NewProjectWizardModal'
import ProjectTable from '../components/ProjectTable'

const { Header, Content } = Layout

function toErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '请求失败'
}

export default function ProjectsPage() {
  const [messageApi, contextHolder] = message.useMessage()

  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const loadedOnceRef = useRef(false)
  const inFlightRef = useRef(false)

  const [newModalOpen, setNewModalOpen] = useState(false)
  const [jobDrawerOpen, setJobDrawerOpen] = useState(false)
  const [activeJobId, setActiveJobId] = useState<string | undefined>(undefined)
  const [configEditorOpen, setConfigEditorOpen] = useState(false)
  const [editingProject, setEditingProject] = useState<Project | null>(null)

  const refreshProjects = useCallback(async () => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    if (!loadedOnceRef.current) setLoading(true)
    setError(null)
    try {
      const res = await api.listProjects()
      setProjects(res.projects)
      loadedOnceRef.current = true
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setLoading(false)
      inFlightRef.current = false
    }
  }, [])

  useEffect(() => {
    void refreshProjects()
    const id = window.setInterval(() => void refreshProjects(), 5000)
    return () => window.clearInterval(id)
  }, [refreshProjects])

  const enqueue = useCallback(
    async (label: string, action: () => Promise<{ job: Job }>) => {
      const key = `job-${Date.now()}`
      messageApi.open({ key, type: 'loading', content: `${label}中…` })
      try {
        const { job } = await action()
        messageApi.open({
          key,
          type: 'success',
          content: `任务已创建：${job.id.slice(0, 8)}…`,
        })
        setActiveJobId(job.id)
        setJobDrawerOpen(true)
        void refreshProjects()
      } catch (err) {
        messageApi.open({ key, type: 'error', content: toErrorMessage(err) })
      }
    },
    [messageApi, refreshProjects],
  )

  const onCreated = useCallback(
    (result: { project: Project; job?: Job }) => {
      setNewModalOpen(false)
      void refreshProjects()
      if (result.job) {
        setActiveJobId(result.job.id)
        setJobDrawerOpen(true)
      } else {
        messageApi.success('项目已创建')
      }
    },
    [messageApi, refreshProjects],
  )

  const onViewLog = useCallback(
    async (project: Project) => {
      try {
        const { job } = await api.getProjectLatestJob(project.id)
        setActiveJobId(job.id)
        setJobDrawerOpen(true)
      } catch (err) {
        messageApi.error('未找到任务日志')
      }
    },
    [messageApi],
  )

  const onEditConfig = useCallback((project: Project) => {
    setEditingProject(project)
    setConfigEditorOpen(true)
  }, [])

  return (
    <>
      {contextHolder}
      <Layout style={{ minHeight: '100vh' }}>
        <Header
          style={{
            position: 'sticky',
            top: 0,
            zIndex: 100,
            paddingInline: 24,
          }}
        >
          <div
            style={{
              maxWidth: 1200,
              margin: '0 auto',
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div
                style={{
                  width: 36,
                  height: 36,
                  borderRadius: 10,
                  background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #06b6d4 100%)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  boxShadow: '0 0 20px rgba(99, 102, 241, 0.4)',
                }}
              >
                <RocketOutlined style={{ fontSize: 18, color: '#fff' }} />
              </div>
              <span
                style={{
                  fontSize: 20,
                  fontWeight: 600,
                  fontFamily: "'Space Grotesk', sans-serif",
                  background: 'linear-gradient(135deg, #f9fafb 0%, #d1d5db 100%)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                }}
              >
                Open Deploy
              </span>
            </div>
            <Space>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => void refreshProjects()}
              >
                刷新
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setNewModalOpen(true)}
              >
                新建项目
              </Button>
            </Space>
          </div>
        </Header>

        <Content style={{ padding: 24 }}>
          <div style={{ maxWidth: 1200, margin: '0 auto' }}>
            {error ? (
              <Alert
                type="error"
                showIcon
                message="加载项目失败"
                description={error}
                style={{ marginBottom: 16 }}
              />
            ) : null}

            <ProjectTable
              projects={projects}
              loading={loading}
              onDeploy={(p) => void enqueue('部署', () => api.deployProject(p.id))}
              onStart={(p) => void enqueue('启动', () => api.startProject(p.id))}
              onStop={(p) => void enqueue('停止', () => api.stopProject(p.id))}
              onPause={(p) => void enqueue('暂停', () => api.pauseProject(p.id))}
              onUnpause={(p) => void enqueue('恢复', () => api.unpauseProject(p.id))}
              onDelete={(p) =>
                void enqueue('删除', () => api.deleteProject(p.id))
              }
              onViewLog={onViewLog}
              onEditConfig={onEditConfig}
            />
          </div>
        </Content>
      </Layout>

      <NewProjectWizardModal
        open={newModalOpen}
        onCancel={() => setNewModalOpen(false)}
        onCreated={onCreated}
      />

      <JobDrawer
        open={jobDrawerOpen}
        jobId={activeJobId}
        onClose={() => setJobDrawerOpen(false)}
      />

      <ConfigEditorModal
        open={configEditorOpen}
        project={editingProject}
        onClose={() => setConfigEditorOpen(false)}
        onSuccess={() => void refreshProjects()}
      />
    </>
  )
}

