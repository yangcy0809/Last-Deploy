import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Descriptions, Drawer, Space, Spin, Typography } from 'antd'
import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import * as api from '../api/openDeploy'
import type { Job } from '../api/types'
import { JobStatusTag } from './StatusTag'

type Props = {
  open: boolean
  jobId?: string
  onClose: () => void
}

function toErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '请求失败'
}

function formatUnixSeconds(ts?: number | null): string {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

export default function JobDrawer({ open, jobId, onClose }: Props) {
  const [job, setJob] = useState<Job | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inFlightRef = useRef(false)

  const fetchJob = useCallback(async () => {
    if (!jobId || inFlightRef.current) return
    inFlightRef.current = true
    setLoading(true)
    setError(null)
    try {
      const res = await api.getJob(jobId)
      setJob(res.job)
    } catch (err) {
      setError(toErrorMessage(err))
    } finally {
      setLoading(false)
      inFlightRef.current = false
    }
  }, [jobId])

  useEffect(() => {
    if (!open || !jobId) return
    setJob(null)
    void fetchJob()
  }, [open, jobId, fetchJob])

  const shouldPoll =
    open &&
    !!jobId &&
    (job == null || job.status === 'queued' || job.status === 'running')

  useEffect(() => {
    if (!shouldPoll) return
    const id = window.setInterval(() => void fetchJob(), 5000)
    return () => window.clearInterval(id)
  }, [shouldPoll, fetchJob])

  return (
    <Drawer
      title="任务详情"
      open={open}
      onClose={onClose}
      width={720}
      extra={
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => void fetchJob()}
            disabled={!jobId}
          >
            刷新
          </Button>
        </Space>
      }
    >
      {error ? <Alert type="error" showIcon message={error} /> : null}

      {loading && !job ? (
        <div style={{ paddingTop: 24 }}>
          <Spin />
        </div>
      ) : null}

      {job ? (
        <>
          <Descriptions
            size="small"
            column={1}
            bordered
            items={[
              {
                key: 'id',
                label: 'Job ID',
                children: <Typography.Text copyable>{job.id}</Typography.Text>,
              },
              {
                key: 'project',
                label: 'Project ID',
                children: (
                  <Typography.Text copyable>{job.project_id}</Typography.Text>
                ),
              },
              { key: 'type', label: '类型', children: job.type },
              { key: 'status', label: '状态', children: <JobStatusTag status={job.status} /> },
              {
                key: 'step',
                label: '当前步骤',
                children: job.current_step?.trim() ? job.current_step : '—',
              },
              { key: 'requested', label: '请求时间', children: formatUnixSeconds(job.requested_at) },
              { key: 'started', label: '开始时间', children: formatUnixSeconds(job.started_at) },
              { key: 'finished', label: '结束时间', children: formatUnixSeconds(job.finished_at) },
            ]}
          />

          <Typography.Title level={5} style={{ marginTop: 16 }}>
            日志
          </Typography.Title>
          <pre
            style={{
              background: '#0b1020',
              color: '#e5e7eb',
              padding: 12,
              borderRadius: 8,
              overflow: 'auto',
              maxHeight: 320,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {job.log?.trim() ? job.log : '暂无日志'}
          </pre>

          {job.error?.trim() ? (
            <>
              <Typography.Title level={5} style={{ marginTop: 16 }}>
                错误
              </Typography.Title>
              <pre
                style={{
                  background: '#fff1f0',
                  color: '#a8071a',
                  padding: 12,
                  borderRadius: 8,
                  overflow: 'auto',
                  maxHeight: 200,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {job.error}
              </pre>
            </>
          ) : null}
        </>
      ) : null}
    </Drawer>
  )
}

