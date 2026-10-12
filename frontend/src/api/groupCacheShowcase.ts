import { apiClient } from './client'
import type { CacheTestRun } from '@/utils/groupCacheTest'

export interface CachePublication {
  id: number
  run_id: string
  created_by: number
  group_id: number
  group_name: string
  model: string
  protocol: string
  tested_at: string
  published_at: string
}
export interface CacheShowcaseItem {
  source?: 'billing' | 'upstream'
  group_id: number
  group_name: string
  platform: string
  model: string
  protocol: string
  last_tested_at: string
  samples: number
  latency_samples: number
  input_tokens: number
  cache_read_tokens: number
  cache_hit_rate: number | null
  first_token_ms: number | null
}
export interface CacheShowcaseView {
  enabled: boolean
  window_start: string
  window_end: string
  items: CacheShowcaseItem[]
}
const adminPath = '/admin/group-cache-showcase'
export async function getCacheShowcase(signal?: AbortSignal): Promise<CacheShowcaseView> {
  return (await apiClient.get<CacheShowcaseView>('/group-cache-showcase', { signal })).data
}
export async function getCacheShowcaseSettings(signal?: AbortSignal): Promise<{ enabled: boolean }> {
  return (await apiClient.get<{ enabled: boolean }>(`${adminPath}/settings`, { signal })).data
}
export async function setCacheShowcaseEnabled(enabled: boolean): Promise<{ enabled: boolean }> {
  return (await apiClient.put<{ enabled: boolean }>(`${adminPath}/settings`, { enabled })).data
}
export async function listCachePublications(groupId: number, signal?: AbortSignal): Promise<CachePublication[]> {
  return (await apiClient.get<CachePublication[]>(`${adminPath}/items`, { params: { group_id: groupId }, signal })).data
}
export function canPublishCacheRun(run: CacheTestRun | null): run is CacheTestRun {
  return !!run && run.status === 'completed' && run.rounds.length === run.settings.rounds &&
    run.rounds.length >= 2 && run.rounds.length <= 6 && run.rounds.every((round, index) =>
      round.index === index + 1 && round.status === 'completed' && round.billingState === 'matched' &&
      round.bill && round.requestId && round.bill.requestId === round.requestId)
}
export async function publishCacheRun(run: CacheTestRun): Promise<CachePublication> {
  if (!canPublishCacheRun(run)) throw new Error('Cache test is not ready to publish')
  // Identities only: the server retrieves the metrics and timestamps itself.
  return (await apiClient.post<CachePublication>(`${adminPath}/items`, {
    run_id: run.id, group_id: run.groupId, api_key_id: run.keyId,
    model: run.settings.model, protocol: run.settings.protocol,
    rounds: run.rounds.map(round => ({ usage_log_id: round.bill!.id, request_id: round.requestId }))
  })).data
}
export async function removeCachePublication(id: number): Promise<void> {
  await apiClient.delete(`${adminPath}/items/${id}`)
}
