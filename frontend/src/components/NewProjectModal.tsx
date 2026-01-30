import {
  Collapse,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Switch,
  message,
} from 'antd'
import { useState } from 'react'
import { ApiError } from '../api/client'
import * as api from '../api/openDeploy'
import type { CreateProjectRequest, DeployType, Job, Project } from '../api/types'

type Props = {
  open: boolean
  onCancel: () => void
  onCreated: (result: { project: Project; job?: Job }) => void
}

type FormValues = {
  name: string
  git_url: string
  host_port: number
  container_port: number
  deploy: boolean

  git_ref?: string
  repo_subdir?: string
  deploy_type: DeployType
  compose_file?: string
  compose_service?: string
  dockerfile_path?: string
}

function toErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '请求失败'
}

function trimOrUndefined(v?: string): string | undefined {
  const t = v?.trim()
  return t ? t : undefined
}

function toRequest(v: FormValues): CreateProjectRequest {
  return {
    name: v.name.trim(),
    git_url: v.git_url.trim(),
    host_port: v.host_port,
    container_port: v.container_port,
    deploy: v.deploy,
    git_ref: trimOrUndefined(v.git_ref),
    repo_subdir: trimOrUndefined(v.repo_subdir),
    deploy_type: v.deploy_type,
    compose_file: trimOrUndefined(v.compose_file),
    compose_service: trimOrUndefined(v.compose_service),
    dockerfile_path: trimOrUndefined(v.dockerfile_path),
  }
}

export default function NewProjectModal({ open, onCancel, onCreated }: Props) {
  const [messageApi, contextHolder] = message.useMessage()
  const [form] = Form.useForm<FormValues>()
  const [submitting, setSubmitting] = useState(false)

  async function submit(values: FormValues) {
    setSubmitting(true)
    try {
      const result = await api.createProject(toRequest(values))
      form.resetFields()
      onCreated(result)
    } catch (err) {
      messageApi.error(toErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      title="新建项目"
      open={open}
      onCancel={onCancel}
      okText="创建"
      cancelText="取消"
      confirmLoading={submitting}
      onOk={() => form.submit()}
      destroyOnClose
    >
      {contextHolder}
      <Form<FormValues>
        form={form}
        layout="vertical"
        onFinish={submit}
        initialValues={{ deploy: true, deploy_type: 'auto' }}
      >
        <Form.Item
          label="名称"
          name="name"
          rules={[{ required: true, message: '请输入名称' }]}
        >
          <Input placeholder="例如: my-service" autoComplete="off" />
        </Form.Item>

        <Form.Item
          label="GitHub URL"
          name="git_url"
          rules={[
            { required: true, message: '请输入 Git URL' },
            { type: 'url', message: '请输入合法 URL' },
          ]}
        >
          <Input placeholder="https://github.com/owner/repo" autoComplete="off" />
        </Form.Item>

        <Form.Item
          label="Host 端口"
          name="host_port"
          rules={[
            { required: true, message: '请输入 Host 端口' },
            { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535' },
          ]}
        >
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          label="容器端口"
          name="container_port"
          rules={[
            { required: true, message: '请输入容器端口' },
            { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535' },
          ]}
        >
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item label="创建后立即部署" name="deploy" valuePropName="checked">
          <Switch />
        </Form.Item>

        <Collapse
          items={[
            {
              key: 'advanced',
              label: '高级选项',
              children: (
                <>
                  <Form.Item label="Git Ref（可选）" name="git_ref">
                    <Input
                      placeholder="main / v1.2.3 / commit sha"
                      autoComplete="off"
                    />
                  </Form.Item>

                  <Form.Item label="Repo 子目录（可选）" name="repo_subdir">
                    <Input placeholder="例如: ./apps/web" autoComplete="off" />
                  </Form.Item>

                  <Form.Item label="部署方式（可选）" name="deploy_type">
                    <Select
                      options={[
                        { value: 'auto', label: 'auto（自动）' },
                        { value: 'dockerfile', label: 'dockerfile' },
                        { value: 'compose', label: 'compose' },
                      ]}
                    />
                  </Form.Item>

                  <Form.Item label="Compose 文件（可选）" name="compose_file">
                    <Input
                      placeholder="例如: docker-compose.yml"
                      autoComplete="off"
                    />
                  </Form.Item>

                  <Form.Item
                    label="Compose 服务名（可选）"
                    name="compose_service"
                    dependencies={['deploy_type', 'compose_file']}
                    rules={[
                      ({ getFieldValue }) => ({
                        validator: async (_rule, value) => {
                          const deployType = getFieldValue(
                            'deploy_type',
                          ) as DeployType
                          const composeFile = String(
                            getFieldValue('compose_file') ?? '',
                          ).trim()
                          const needsService =
                            deployType === 'compose' || composeFile !== ''
                          if (!needsService) return
                          if (String(value ?? '').trim() === '') {
                            throw new Error('compose_service 不能为空')
                          }
                        },
                      }),
                    ]}
                  >
                    <Input placeholder="例如: web" autoComplete="off" />
                  </Form.Item>

                  <Form.Item label="Dockerfile 路径（可选）" name="dockerfile_path">
                    <Input placeholder="例如: ./Dockerfile" autoComplete="off" />
                  </Form.Item>
                </>
              ),
            },
          ]}
        />
      </Form>
    </Modal>
  )
}
