import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { PelicanGroupTestAttempt } from './pelicanTests'

export const CACHE_TEST_PROMPT = 'Read the synthetic reference below. Reply with OK only.'
export interface CachePlanConfig {
  prompt: string
  reasoning_effort: string
  parallel_count: number
  rounds: number
  prefix_blocks: number
  max_output_tokens: number
  interval_ms: number
}
export interface CachePlanInput extends CachePlanConfig {
  group_id: number
  model_id: string
  cron_expression: string
  enabled: boolean
}
export interface CachePlanRound {
  index: number
  status: string
  protocol: string
  input_tokens: number | null
  cache_read_tokens: number | null
  cache_write_tokens: number | null
  output_tokens: number | null
  first_token_ms: number | null
  duration_ms: number
  measured_at: string
}
export interface CachePlanResult {
  id: number
  plan_id: number
  group_id: number
  group_name: string
  model_id: string
  account_id: number
  account_name: string
  attempts: PelicanGroupTestAttempt[]
  status: 'success' | 'failed'
  error_message: string
  latency_ms: number
  cost_usd: number | null
  cost_incomplete: boolean
  config: CachePlanConfig
  rounds: CachePlanRound[]
  publication_id: number | null
  started_at: string
  finished_at: string
}
export interface CacheTestPlan {
  id: number
  group_id: number
  group_name: string
  group_platform: string
  group_status: string
  today_cost_usd: number
  total_cost_usd: number
  today_cost_incomplete: boolean
  total_cost_incomplete: boolean
  model_id: string
  cron_expression: string
  enabled: boolean
  config: CachePlanConfig
  last_run_at: string | null
  next_run_at: string | null
  running_until: string | null
  last_result?: CachePlanResult
  updated_at: string
}
export interface CacheShowcaseSettings {
  enabled: boolean
  api_enabled: boolean
  max_items: number
  auto_cleanup: boolean
  retention_days: number
}
const plansPath = '/admin/group-cache-tests'
const resultsPath = '/admin/group-cache-test-results'
const settingsPath = '/admin/group-cache-showcase/settings'
export const cacheTestsAPI = {
  async listPlans(): Promise<CacheTestPlan[]> { return (await apiClient.get<CacheTestPlan[]>(plansPath)).data ?? [] },
  async createPlan(input: CachePlanInput): Promise<CacheTestPlan> { return (await apiClient.post<CacheTestPlan>(plansPath, input)).data },
  async updatePlan(id: number, input: CachePlanInput): Promise<CacheTestPlan> { return (await apiClient.put<CacheTestPlan>(`${plansPath}/${id}`, input)).data },
  async deletePlan(id: number): Promise<void> { await apiClient.delete(`${plansPath}/${id}`) },
  async runPlan(id: number): Promise<void> { await apiClient.post(`${plansPath}/${id}/run`) },
  async listResults(page = 1, pageSize = 20, planId = 0, signal?: AbortSignal): Promise<PaginatedResponse<CachePlanResult>> {
    return (await apiClient.get<PaginatedResponse<CachePlanResult>>(resultsPath, { params: { page, page_size: pageSize, plan_id: planId || undefined }, signal })).data
  },
  async getResult(id: number): Promise<CachePlanResult> { return (await apiClient.get<CachePlanResult>(`${resultsPath}/${id}`)).data },
  async getShowcaseSettings(): Promise<CacheShowcaseSettings> { return (await apiClient.get<CacheShowcaseSettings>(settingsPath)).data },
  async updateShowcaseSettings(settings: CacheShowcaseSettings): Promise<CacheShowcaseSettings> { return (await apiClient.put<CacheShowcaseSettings>(settingsPath, settings)).data },
  async withdraw(id: number): Promise<void> { await apiClient.delete(`/admin/group-cache-showcase/items/${id}`) },
}
