import {
  Collapse,
  Form,
  Input,
  Modal,
  Space,
  Steps,
  Switch,
  Tag,
  message,
} from 'antd'
import { useMemo, useState } from 'react'
import { ApiError } from '../api/client'
import * as api from '../api/openDeploy'
import type { DetectProjectResponse, Job, Project } from '../api/types'

type Props = {
  open: boolean
  onCancel: () => void
  onCreated: (result: { project: Project; job?: Job }) => void
}

type Step1Values = {
  name: string
  git_url: string
}

type Step2Values = {
  dockerfile_content: string
  compose_content: string
  compose_service?: string
  git_ref?: string
  repo_subdir?: string
  deploy: boolean
}

function toErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '请求失败'
}

export default function NewProjectWizardModal({ open, onCancel, onCreated }: Props) {
  const [messageApi, contextHolder] = message.useMessage()
  const [step, setStep] = useState(0)
  const [loading, setLoading] = useState(false)

  const [step1Form] = Form.useForm<Step1Values>()
  const [step2Form] = Form.useForm<Step2Values>()

  const [detectResult, setDetectResult] = useState<DetectProjectResponse | null>(null)

  // 监听 compose_content 变化，用于 none 模式下动态显示 service 输入框
  const composeContentValue = Form.useWatch('compose_content', step2Form)
  const showServiceInput = useMemo(() => {
    if (!detectResult) return false
    // compose 类型始终显示
    if (detectResult.deploy_type === 'compose') return true
    // none 类型且用户填写了 compose 内容
    if (detectResult.deploy_type === 'none' && composeContentValue?.trim()) return true
    return false
  }, [detectResult, composeContentValue])

  const reset = () => {
    setStep(0)
    setDetectResult(null)
    step1Form.resetFields()
    step2Form.resetFields()
  }

  const handleCancel = () => {
    reset()
    onCancel()
  }

  const handleDetect = async () => {
    const values = await step1Form.validateFields()
    setLoading(true)
    try {
      const result = await api.detectProject({
        name: values.name.trim(),
        git_url: values.git_url.trim(),
      })
      setDetectResult(result)
      step2Form.setFieldsValue({
        dockerfile_content: result.dockerfile_content,
        compose_content: result.deploy_type === 'none' ? '' : result.compose_content,
        deploy: true,
      })
      setStep(1)
    } catch (err) {
      messageApi.error(toErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = async () => {
    if (!detectResult) return
    const values = await step2Form.validateFields()
    setLoading(true)
    try {
      const result = await api.createProjectFromDraft({
        draft_id: detectResult.draft_id,
        dockerfile_content: values.dockerfile_content,
        compose_content: values.compose_content,
        compose_service: values.compose_service?.trim() || undefined,
        git_ref: values.git_ref?.trim() || undefined,
        repo_subdir: values.repo_subdir?.trim() || undefined,
        deploy: values.deploy,
      })
      reset()
      onCreated(result)
    } catch (err) {
      messageApi.error(toErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      title="新建项目"
      open={open}
      onCancel={handleCancel}
      okText={step === 0 ? '检测' : '创建'}
      cancelText="取消"
      confirmLoading={loading}
      onOk={step === 0 ? handleDetect : handleCreate}
      destroyOnClose
      width={600}
    >
      {contextHolder}
      <Steps
        current={step}
        items={[{ title: '基本信息' }, { title: '配置确认' }]}
        style={{ marginBottom: 24 }}
      />

      {step === 0 && (
        <Form<Step1Values> form={step1Form} layout="vertical">
          <Form.Item
            label="名称"
            name="name"
            rules={[{ required: true, message: '请输入名称' }]}
          >
            <Input placeholder="例如: my-service" autoComplete="off" />
          </Form.Item>
          <Form.Item
            label="Git URL"
            name="git_url"
            rules={[
              { required: true, message: '请输入 Git URL' },
              { type: 'url', message: '请输入合法 URL' },
            ]}
          >
            <Input placeholder="https://github.com/owner/repo" autoComplete="off" />
          </Form.Item>
        </Form>
      )}

      {step === 1 && detectResult && (
        <Form<Step2Values> form={step2Form} layout="vertical" initialValues={{ deploy: true }}>
          <Space style={{ marginBottom: 16 }}>
            <span>检测结果:</span>
            <Tag color={detectResult.deploy_type === 'none' ? 'red' : 'blue'}>
              {detectResult.deploy_type}
            </Tag>
            {detectResult.dockerfile_path && (
              <Tag>{detectResult.dockerfile_path}</Tag>
            )}
            {detectResult.compose_path && (
              <Tag>{detectResult.compose_path}</Tag>
            )}
          </Space>

          {(detectResult.services?.length ?? 0) > 0 && (
            <div style={{ marginBottom: 16 }}>
              <span>服务列表: </span>
              {(detectResult.services ?? []).map((s) => (
                <Tag key={s}>{s}</Tag>
              ))}
            </div>
          )}

          <Form.Item
            label="Dockerfile 内容"
            name="dockerfile_content"
            rules={[{ required: true, message: 'Dockerfile 内容不能为空' }]}
          >
            <Input.TextArea rows={8} style={{ fontFamily: 'monospace' }} />
          </Form.Item>

          <Form.Item
            label="Docker Compose 内容"
            name="compose_content"
            tooltip={detectResult.deploy_type === 'none' ? '选填，填写后将使用 compose 方式部署' : undefined}
            rules={[{ required: detectResult.deploy_type === 'compose', message: 'Compose 内容不能为空' }]}
          >
            <Input.TextArea rows={8} style={{ fontFamily: 'monospace' }} placeholder={detectResult.deploy_type === 'none' ? '选填，若填写则使用 compose 方式部署' : undefined} />
          </Form.Item>

          {showServiceInput && (
            <Form.Item
              label="Compose 服务名"
              name="compose_service"
              tooltip="可以填写单个服务名（如 web）、多个服务名（如 web,db,redis）或留空部署所有服务"
            >
              <Input placeholder="例如: web 或 web,db,redis（留空部署全部）" autoComplete="off" />
            </Form.Item>
          )}

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
                      <Input placeholder="main / v1.2.3 / commit sha" autoComplete="off" />
                    </Form.Item>
                    <Form.Item label="Repo 子目录（可选）" name="repo_subdir">
                      <Input placeholder="例如: ./apps/web" autoComplete="off" />
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />
        </Form>
      )}
    </Modal>
  )
}
