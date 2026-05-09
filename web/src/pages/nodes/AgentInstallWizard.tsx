import React, { useEffect, useRef, useState } from 'react'
import { Modal, Steps, Button, Space, Message, Spin } from '@arco-design/web-react'
import { Step1NodeName, type Mode } from './wizard/Step1NodeName'
import { Step2DeployOptions, type DeployOptions } from './wizard/Step2DeployOptions'
import { Step3CommandPreview } from './wizard/Step3CommandPreview'
import { BatchCommandTable, type BatchCommandRow } from './BatchCommandTable'
import type { InstallTokenResult } from '../../types/nodes'
import { useAgentDeployFlow, type AgentDeployRow } from './useAgentDeployFlow'

const Step = Steps.Step

interface Props {
  visible: boolean
  onClose: () => void
  onSuccess: () => void
  // null = 正在拉取；空字符串 = 拉取失败（Step2 将展示手动输入框而非 Select）
  masterVersion: string | null
  // 当从节点列表直接点"生成安装命令"时传入，跳过 Step1
  fixedNode?: { id: number; name: string }
}

export function AgentInstallWizard({ visible, onClose, onSuccess, masterVersion, fixedNode }: Props) {
  const [step, setStep] = useState(fixedNode ? 1 : 0)
  const [mode, setMode] = useState<Mode>('single')
  const [singleName, setSingleName] = useState('')
  const [batchText, setBatchText] = useState('')
  const deployFlow = useAgentDeployFlow()

  const [deploy, setDeploy] = useState<DeployOptions>({
    mode: 'systemd',
    arch: 'auto',
    agentVersion: masterVersion || '',
    downloadSrc: 'github',
    ttlSeconds: 900,
  })

  // 当父组件异步拿到 masterVersion 后，同步到 deploy.agentVersion（仅初始为空时）
  useEffect(() => {
    if (masterVersion && !deploy.agentVersion) {
      setDeploy((prev) => ({ ...prev, agentVersion: masterVersion }))
    }
  }, [masterVersion]) // eslint-disable-line react-hooks/exhaustive-deps

  // unmount 保护：用户关 Modal / 切页时，已发出的请求完成后不再更新 state
  const mountedRef = useRef(true)
  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  const [singleToken, setSingleToken] = useState<InstallTokenResult | null>(null)
  const [singleNodeInfo, setSingleNodeInfo] = useState<{ id: number; name: string } | null>(null)
  const [batchRows, setBatchRows] = useState<BatchCommandRow[]>([])
  const [submitting, setSubmitting] = useState(false)

  const reset = () => {
    setStep(fixedNode ? 1 : 0)
    setMode('single')
    setSingleName('')
    setBatchText('')
    setSingleToken(null)
    setSingleNodeInfo(null)
    setBatchRows([])
  }

  const handleClose = () => {
    reset()
    onClose()
  }

  const parseBatchNames = (): string[] =>
    batchText.split('\n').map((s) => s.trim()).filter(Boolean)

  const handleNextFromStep1 = () => {
    if (mode === 'single') {
      if (!singleName.trim()) {
        Message.warning('请输入节点名称')
        return
      }
    } else {
      const names = parseBatchNames()
      if (names.length === 0) {
        Message.warning('请至少输入一个节点名称')
        return
      }
      if (names.length > 50) {
        Message.warning('单次最多创建 50 个节点')
        return
      }
    }
    setStep(1)
  }

  const handleGenerate = async () => {
    if (!deploy.agentVersion.trim()) {
      Message.warning('请填写 Agent 版本号（形如 v1.7.0）')
      return
    }
    setSubmitting(true)
    try {
      if (fixedNode) {
        const result = await deployFlow.submitExistingNode(fixedNode, deploy)
        applySingleOrTableResult(result.rows, fixedNode)
      } else if (mode === 'single') {
        const result = await deployFlow.submitNewNodes([singleName.trim()], deploy)
        applySingleOrTableResult(result.rows)
      } else {
        const names = parseBatchNames()
        const result = await deployFlow.submitNewNodes(names, deploy)
        if (mountedRef.current) setBatchRows(toBatchRows(result.rows))
        if (result.status === 'partialFailed') {
          Message.warning('部分节点安装命令生成失败，可在结果表中查看')
        }
      }
      setStep(2)
      onSuccess()
    } catch (e: any) {
      Message.error(e?.message || '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  const regenerateSingle = async () => {
    if (!singleNodeInfo) return
    setSubmitting(true)
    try {
      const row = await deployFlow.regenerateNode(singleNodeInfo, deploy)
      if (row.status === 'ready' && row.installToken) {
        setSingleToken(row.installToken)
      } else {
        Message.error(row.errorMessage || '重新生成失败')
      }
    } catch (e: any) {
      Message.error(e?.message || '重新生成失败')
    } finally {
      setSubmitting(false)
    }
  }

  const retryBatchNode = async (row: BatchCommandRow) => {
    setSubmitting(true)
    try {
      const next = await deployFlow.regenerateNode({ id: row.nodeId, name: row.nodeName }, deploy)
      setBatchRows((rows) => rows.map((item) => (
        item.nodeId === row.nodeId ? toBatchRows([next])[0] : item
      )))
      if (next.status === 'ready') {
        Message.success(`节点「${row.nodeName}」安装命令已重新生成`)
      } else {
        Message.error(next.errorMessage || '重试失败')
      }
    } catch (e: any) {
      Message.error(e?.message || '重试失败')
    } finally {
      setSubmitting(false)
    }
  }

  const previewParams = {
    mode: deploy.mode,
    arch: deploy.arch,
    agentVersion: deploy.agentVersion,
    downloadSrc: deploy.downloadSrc,
  }

  // fixedNode 路径下步骤只有 2 步（部署参数 + 安装命令），step 值从 1 开始，
  // 需要映射到 Steps current（0-based）
  const stepsCurrent = fixedNode ? step - 1 : step

  return (
    <Modal
      title={fixedNode ? `为「${fixedNode.name}」生成安装命令` : '添加节点'}
      visible={visible}
      onCancel={handleClose}
      footer={null}
      style={{ width: 760 }}
      unmountOnExit
    >
      <Steps current={stepsCurrent} size="small" style={{ marginBottom: 24 }}>
        {!fixedNode && <Step title="节点信息" />}
        <Step title="部署参数" />
        <Step title="安装命令" />
      </Steps>

      {submitting && (
        <div style={{ textAlign: 'center', padding: 32 }}>
          <Spin />
        </div>
      )}

      {!submitting && step === 0 && (
        <>
          <Step1NodeName
            mode={mode}
            onModeChange={setMode}
            singleName={singleName}
            onSingleNameChange={setSingleName}
            batchText={batchText}
            onBatchTextChange={setBatchText}
          />
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              <Button onClick={handleClose}>取消</Button>
              <Button type="primary" onClick={handleNextFromStep1}>
                下一步
              </Button>
            </Space>
          </div>
        </>
      )}

      {!submitting && step === 1 && (
        <>
          <Step2DeployOptions masterVersion={masterVersion} value={deploy} onChange={setDeploy} />
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              {!fixedNode && (
                <Button onClick={() => setStep(0)}>上一步</Button>
              )}
              <Button onClick={handleClose}>取消</Button>
              <Button type="primary" onClick={handleGenerate} loading={submitting}>
                生成安装命令
              </Button>
            </Space>
          </div>
        </>
      )}

      {!submitting && step === 2 && (
        <>
          {singleToken && singleNodeInfo && (
            <Step3CommandPreview
              nodeId={singleNodeInfo.id}
              nodeName={singleNodeInfo.name}
              token={singleToken}
              mode={deploy.mode}
              previewParams={previewParams}
              onRegenerate={regenerateSingle}
            />
          )}
          {batchRows.length > 0 && <BatchCommandTable rows={batchRows} onRetryNode={retryBatchNode} />}
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Button type="primary" onClick={handleClose}>
              完成
            </Button>
          </div>
        </>
      )}
    </Modal>
  )

  function applySingleOrTableResult(rows: AgentDeployRow[], fallbackNode?: { id: number; name: string }) {
    const row = rows[0]
    if (!row) return
    if (row.status === 'ready' && row.installToken) {
      setSingleNodeInfo({ id: row.nodeId || fallbackNode?.id || 0, name: row.nodeName || fallbackNode?.name || '' })
      setSingleToken(row.installToken)
      setBatchRows([])
      return
    }
    setSingleNodeInfo(null)
    setSingleToken(null)
    setBatchRows(toBatchRows(rows))
    Message.error(row.errorMessage || '安装命令生成失败')
  }
}

function toBatchRows(rows: AgentDeployRow[]): BatchCommandRow[] {
  return rows.map((row) => ({
    nodeId: row.nodeId,
    nodeName: row.nodeName,
    status: row.status,
    command: row.command,
    expiresAt: row.expiresAt,
    errorMessage: row.errorMessage,
    embeddedCommand: row.embeddedCommand,
  }))
}
