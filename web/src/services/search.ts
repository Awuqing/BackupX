import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'

export type SearchKind = 'task' | 'record' | 'storage' | 'node'

export interface SearchResultItem {
  kind: SearchKind
  id: number
  title: string
  subtitle?: string
  highlight?: string
  url: string
}

export interface SearchResult {
  query: string
  tasks: SearchResultItem[]
  records: SearchResultItem[]
  storage: SearchResultItem[]
  nodes: SearchResultItem[]
  totalCount: number
}

export async function globalSearch(query: string): Promise<SearchResult> {
  const response = await http.get<ApiEnvelope<SearchResult>>('/search', { params: { q: query } })
  return unwrapApiEnvelope(response.data)
}
