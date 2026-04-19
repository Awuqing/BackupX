import React, { useState } from 'react'
import { Modal, Steps, Button, Space, Message, Spin } from '@arco-design/web-react'
import { Step1NodeName, type Mode } from './wizard/Step1NodeName'
import { Step2DeployOptions, type DeployOptions } from './wizard/Step2DeployOptions'
import { Step3CommandPreview } from './wizard/Step3CommandPreview'
import { BatchCommandTable, type BatchCommandRow } from './BatchCommandTable'
import { batchCreateNodes, createInstallToken } from '../../services/nodes'
import type { InstallTokenResult } from '../../types/nodes'

const Step = Steps.Step

interface Props {
  visible: boolean
  onClose: () => void
  onSuccess: () => void
  masterVersion: string
  // 当从节点列表直接点"生成安装命令"时传入，跳过 Step1
  fixedNode?: { id: number; name: string }
}

export function AgentInstallWizard({ visible, onClose, onSuccess, masterVersion, fixedNode }: Props) {
  const [step, setStep] = useState(fixedNode ? 1 : 0)
  const [mode, setMode] = useState<Mode>('single')
  const [singleName, setSingleName] = useState('')
  const [batchText, setBatchText] = useState('')

  const [deploy, setDeploy] = useState<DeployOptions>({
    mode: 'systemd',
    arch: 'auto',
    agentVersion: masterVersion,
    downloadSrc: 'github',
    ttlSeconds: 900,
  })

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
    setSubmitting(true)
    try {
      if (fixedNode) {
        const tok = await createInstallToken(fixedNode.id, {
          mode: deploy.mode,
          arch: deploy.arch,
          agentVersion: deploy.agentVersion,
          downloadSrc: deploy.downloadSrc,
          ttlSeconds: deploy.ttlSeconds,
        })
        setSingleNodeInfo(fixedNode)
        setSingleToken(tok)
      } else if (mode === 'single') {
        const created = await batchCreateNodes([singleName.trim()])
        const one = created[0]
        const tok = await createInstallToken(one.id, {
          mode: deploy.mode,
          arch: deploy.arch,
          agentVersion: deploy.agentVersion,
          downloadSrc: deploy.downloadSrc,
          ttlSeconds: deploy.ttlSeconds,
        })
        setSingleNodeInfo({ id: one.id, name: one.name })
        setSingleToken(tok)
      } else {
        const names = parseBatchNames()
        const created = await batchCreateNodes(names)
        const rows: BatchCommandRow[] = []
        for (const c of created) {
          const tok = await createInstallToken(c.id, {
            mode: deploy.mode,
            arch: deploy.arch,
            agentVersion: deploy.agentVersion,
            downloadSrc: deploy.downloadSrc,
            ttlSeconds: deploy.ttlSeconds,
          })
          rows.push({
            nodeId: c.id,
            nodeName: c.name,
            command: `curl -fsSL ${tok.url} | sudo sh`,
            expiresAt: tok.expiresAt,
          })
        }
        setBatchRows(rows)
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
      const tok = await createInstallToken(singleNodeInfo.id, {
        mode: deploy.mode,
        arch: deploy.arch,
        agentVersion: deploy.agentVersion,
        downloadSrc: deploy.downloadSrc,
        ttlSeconds: deploy.ttlSeconds,
      })
      setSingleToken(tok)
    } catch (e: any) {
      Message.error(e?.message || '重新生成失败')
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
          {batchRows.length > 0 && <BatchCommandTable rows={batchRows} />}
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Button type="primary" onClick={handleClose}>
              完成
            </Button>
          </div>
        </>
      )}
    </Modal>
  )
}
