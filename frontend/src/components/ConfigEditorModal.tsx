import { Modal, Input, message, Tabs } from 'antd'
import { useState } from 'react'
import * as api from '../api/openDeploy'
import type { Project } from '../api/types'

interface ConfigEditorModalProps {
  open: boolean
  project: Project | null
  onClose: () => void
  onSuccess: () => void
}

export default function ConfigEditorModal({
  open,
  project,
  onClose,
  onSuccess,
}: ConfigEditorModalProps) {
  const [dockerfileContent, setDockerfileContent] = useState('')
  const [composeContent, setComposeContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [activeTab, setActiveTab] = useState('dockerfile')

  const handleOpen = () => {
    if (project) {
      setDockerfileContent(project.dockerfile_content || '')
      setComposeContent(project.compose_content || '')
      // 根据 deploy_type 设置默认 tab
      setActiveTab(project.deploy_type === 'compose' ? 'compose' : 'dockerfile')
    }
  }

  const handleSave = async () => {
    if (!project) return

    setLoading(true)
    try {
      await api.updateProjectConfig(project.id, dockerfileContent, composeContent)
      message.success('配置已更新')
      onSuccess()
      onClose()
    } catch (err) {
      message.error('更新失败：' + (err instanceof Error ? err.message : '未知错误'))
    } finally {
      setLoading(false)
    }
  }

  const title = project ? `编辑配置 - ${project.name}` : '编辑配置'

  const tabItems = [
    {
      key: 'dockerfile',
      label: 'Dockerfile',
      children: (
        <Input.TextArea
          value={dockerfileContent}
          onChange={(e) => setDockerfileContent(e.target.value)}
          rows={18}
          style={{ fontFamily: 'monospace', fontSize: 13 }}
          placeholder="输入 Dockerfile 内容..."
        />
      ),
    },
    {
      key: 'compose',
      label: 'docker-compose.yml',
      children: (
        <Input.TextArea
          value={composeContent}
          onChange={(e) => setComposeContent(e.target.value)}
          rows={18}
          style={{ fontFamily: 'monospace', fontSize: 13 }}
          placeholder="输入 docker-compose.yml 内容..."
        />
      ),
    },
  ]

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      onOk={handleSave}
      afterOpenChange={(visible) => visible && handleOpen()}
      confirmLoading={loading}
      width={800}
      okText="保存"
      cancelText="取消"
    >
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={tabItems}
      />
    </Modal>
  )
}
