import { useMemo } from 'react'
import type { BatchCreateResult, InstallTokenInput, InstallTokenResult } from '../../types/nodes'
import { batchCreateNodes, createInstallToken } from '../../services/nodes'
import {
  buildAgentInstallCommand,
  buildEmbeddedAgentInstallCommand,
} from './installCommands'

export type DeployRowStatus = 'ready' | 'failed'
export type DeployResultStatus = 'ready' | 'partialFailed'

export interface AgentDeployNode {
  id: number
  name: string
}

export interface AgentDeployRow {
  nodeId: number
  nodeName: string
  status: DeployRowStatus
  command: string
  expiresAt: string
  installToken?: InstallTokenResult
  embeddedCommand?: string
  errorMessage?: string
}

export interface AgentDeployResult {
  status: DeployResultStatus
  rows: AgentDeployRow[]
}

interface AgentDeployFlowDeps {
  batchCreateNodes: (names: string[]) => Promise<BatchCreateResult[]>
  createInstallToken: (nodeId: number, input: InstallTokenInput) => Promise<InstallTokenResult>
}

const TOKEN_CONCURRENCY = 4

export function createAgentDeployFlow(deps: AgentDeployFlowDeps) {
  const issueTokenForNode = async (node: AgentDeployNode, input: InstallTokenInput): Promise<AgentDeployRow> => {
    try {
      const token = await deps.createInstallToken(node.id, input)
      return readyRow(node, token)
    } catch (error) {
      return {
        nodeId: node.id,
        nodeName: node.name,
        status: 'failed',
        command: '',
        expiresAt: '',
        errorMessage: resolveErrorMessage(error),
      }
    }
  }

  return {
    async submitNewNodes(names: string[], input: InstallTokenInput): Promise<AgentDeployResult> {
      const cleanedNames = normalizeNodeNames(names)
      const nodes = await deps.batchCreateNodes(cleanedNames)
      const rows = await mapWithConcurrency(nodes, TOKEN_CONCURRENCY, (node) => issueTokenForNode(node, input))
      return resultFromRows(rows)
    },

    async submitExistingNode(node: AgentDeployNode, input: InstallTokenInput): Promise<AgentDeployResult> {
      const row = await issueTokenForNode(node, input)
      return resultFromRows([row])
    },

    async regenerateNode(node: AgentDeployNode, input: InstallTokenInput): Promise<AgentDeployRow> {
      return issueTokenForNode(node, input)
    },
  }
}

export function useAgentDeployFlow() {
  return useMemo(() => createAgentDeployFlow({ batchCreateNodes, createInstallToken }), [])
}

function readyRow(node: AgentDeployNode, token: InstallTokenResult): AgentDeployRow {
  return {
    nodeId: node.id,
    nodeName: node.name,
    status: 'ready',
    command: buildAgentInstallCommand(token.url, token.fallbackUrl),
    expiresAt: token.expiresAt,
    installToken: token,
    embeddedCommand: token.scriptBase64
      ? buildEmbeddedAgentInstallCommand(token.scriptBase64)
      : undefined,
  }
}

function resultFromRows(rows: AgentDeployRow[]): AgentDeployResult {
  return {
    status: rows.some((row) => row.status === 'failed') ? 'partialFailed' : 'ready',
    rows,
  }
}

function normalizeNodeNames(names: string[]) {
  const cleaned = names.map((name) => name.trim()).filter(Boolean)
  if (cleaned.length === 0) {
    throw new Error('请至少输入一个节点名称')
  }
  if (cleaned.length > 50) {
    throw new Error('单次最多创建 50 个节点')
  }
  const seen = new Set<string>()
  for (const name of cleaned) {
    if (seen.has(name)) {
      throw new Error(`批次内重复节点名：${name}`)
    }
    seen.add(name)
  }
  return cleaned
}

async function mapWithConcurrency<T, R>(
  items: T[],
  concurrency: number,
  mapper: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  const results = new Array<R>(items.length)
  let nextIndex = 0
  const workerCount = Math.min(concurrency, items.length)
  const workers = Array.from({ length: workerCount }, async () => {
    for (;;) {
      const index = nextIndex
      nextIndex += 1
      if (index >= items.length) {
        return
      }
      results[index] = await mapper(items[index], index)
    }
  })
  await Promise.all(workers)
  return results
}

function resolveErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message
  }
  return '生成安装命令失败'
}
