import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import View from '../GroupCacheTestView.vue'
import type { CacheTestPlan, CachePlanResult, CachePlanConfig } from '@/api/admin/groupCacheTests'
const api = vi.hoisted(() => ({ listPlans: vi.fn(), createPlan: vi.fn(), updatePlan: vi.fn(), deletePlan: vi.fn(), runPlan: vi.fn(), listResults: vi.fn(), getResult: vi.fn(), getShowcaseSettings: vi.fn(), updateShowcaseSettings: vi.fn(), withdraw: vi.fn() }))
const refs = vi.hoisted(() => ({ groups: vi.fn(), publicSettings: vi.fn() }))
vi.mock('@/api/admin/groupCacheTests', () => ({ cacheTestsAPI: api, CACHE_TEST_PROMPT: 'Read the synthetic reference below. Reply with OK only.' }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: refs.groups } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: refs.publicSettings }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, named?: Record<string, unknown>) => named ? `${key} ${JSON.stringify(named)}` : key }) }))
const config: CachePlanConfig = { prompt: 'Synthetic test', reasoning_effort: 'minimal', parallel_count: 1, rounds: 3, prefix_blocks: 32, interval_ms: 1000, max_output_tokens: 256 }
const result: CachePlanResult = { id: 9, plan_id: 7, group_id: 4, group_name: 'Demo group', model_id: 'demo', account_id: 11, account_name: 'fixture account', attempts: [], status: 'success', error_message: '', latency_ms: 1200, cost_usd: null, cost_incomplete: true, config, rounds: [{ index: 1, protocol: 'responses', status: 'success', input_tokens: 1000, output_tokens: 2, cache_read_tokens: 0, cache_write_tokens: 0, first_token_ms: 4700, duration_ms: 4900, measured_at: '2026-10-12T10:00:00Z' }], publication_id: 90, started_at: '2026-10-12T10:00:00Z', finished_at: '2026-10-12T10:01:00Z' }
const plan: CacheTestPlan = { id: 7, group_id: 4, group_name: 'Demo group', group_platform: 'openai', group_status: 'active', today_cost_usd: 0, total_cost_usd: 0.5, today_cost_incomplete: false, total_cost_incomplete: true, model_id: 'demo', cron_expression: '*/30 * * * *', enabled: true, config, last_run_at: null, next_run_at: '2026-10-12T10:30:00Z', running_until: null, updated_at: '', last_result: result }
const settings = { enabled: false, api_enabled: false, max_items: 20, auto_cleanup: true, retention_days: 7 }
const SelectStub = { props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'], template: `<select :disabled="disabled" :value="modelValue" @change="$emit('update:modelValue', options.find(o => String(o.value) === $event.target.value)?.value)"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>` }
const mountView = () => mount(View, { global: { stubs: {
  AppLayout: { template: '<div><slot /></div>' }, SmartOpsNav: true, RouterLink: RouterLinkStub, Icon: true, PlatformIcon: true, Select: SelectStub,
  Toggle: { props: ['modelValue'], emits: ['update:modelValue'], template: `<input type="checkbox" :checked="modelValue" @change="$emit('update:modelValue', $event.target.checked)" />` },
  BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  ConfirmDialog: { props: ['show'], emits: ['confirm'], template: `<button v-if="show" data-testid="confirm-delete" @click="$emit('confirm')" />` },
} } })
let wrapper: ReturnType<typeof mountView>
beforeEach(() => {
  vi.useFakeTimers()
  for (const fn of Object.values(api)) fn.mockReset()
  api.listPlans.mockResolvedValue([{ ...plan }]); api.getShowcaseSettings.mockResolvedValue({ ...settings }); api.listResults.mockResolvedValue({ items: [result], total: 1 }); api.getResult.mockResolvedValue(result)
  refs.groups.mockReset().mockResolvedValue([{ id: 4, name: 'Demo group', status: 'active', platform: 'openai' }]); refs.publicSettings.mockReset().mockResolvedValue(undefined)
})
afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })
describe('Group cache plans following the Pelican flow', () => {
  it('shows saved plans, costs, next run and a modal without requiring a personal key', async () => {
    wrapper = mountView(); await flushPromises()
    expect(wrapper.get('[data-testid="cache-plan-7"]').text()).toContain('fixture account')
    expect(wrapper.get('[data-testid="cache-plan-cost-7"]').text()).toContain('$0.500000')
    await wrapper.get('[data-testid="cache-tests-create"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-testid="cache-test-editor"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="cache-test-sampling"]').exists()).toBe(true)
    expect(wrapper.findAll('input').some(input => input.attributes('name') === 'api_key')).toBe(false)
    await wrapper.get('[data-testid="cache-test-editor"] select').setValue('4')
    await wrapper.get('[data-testid="cache-test-model"]').setValue('demo')
    await wrapper.get('[data-testid="cache-test-editor"]').trigger('submit'); await flushPromises()
    expect(api.createPlan).toHaveBeenCalledWith(expect.objectContaining({ group_id: 4, model_id: 'demo', rounds: 3, parallel_count: 1, cron_expression: '*/30 * * * *', enabled: true }))
    expect(api.runPlan).not.toHaveBeenCalled()
  })
  it('saves showcase and retention together and keeps unsaved settings across refreshes', async () => {
    wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="cache-showcase-max-items"]').setValue(30)
    await wrapper.get('[data-testid="cache-showcase-enabled"]').setValue(true)
    await wrapper.get('[data-testid="cache-showcase-api-enabled"]').setValue(true)
    await wrapper.get('.ops-heading button.btn-secondary').trigger('click'); await flushPromises()
    expect((wrapper.get('[data-testid="cache-showcase-max-items"]').element as HTMLInputElement).value).toBe('30')
    api.updateShowcaseSettings.mockResolvedValue({ ...settings, enabled: true, api_enabled: true, max_items: 30 })
    await wrapper.get('.settings-form').trigger('submit'); await flushPromises()
    expect(api.updateShowcaseSettings).toHaveBeenCalledWith({ ...settings, enabled: true, api_enabled: true, max_items: 30 })
    expect(refs.publicSettings).toHaveBeenCalledWith(true)
  })
  it('runs once, pauses, edits a fixed group and deletes through confirmation', async () => {
    wrapper = mountView(); await flushPromises()
    expect(wrapper.findComponent(RouterLinkStub).props('to')).toBe('/group-cache-showcase')
    await wrapper.get('[data-testid="cache-plan-run-7"]').trigger('click'); await flushPromises(); expect(api.runPlan).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-testid="cache-plan-toggle-7"]').trigger('click'); await flushPromises(); expect(api.updatePlan).toHaveBeenCalledWith(7, expect.objectContaining({ enabled: false, ...config }))
    await wrapper.get('[data-testid="cache-plan-edit-7"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-testid="cache-test-editor"] select').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="cache-test-prompt"]').setValue('Changed synthetic instruction')
    await wrapper.get('[data-testid="cache-test-editor"]').trigger('submit'); await flushPromises()
    expect(api.updatePlan).toHaveBeenCalledWith(7, expect.objectContaining({ group_id: 4, prompt: 'Changed synthetic instruction' }))
    await wrapper.get('[data-testid="cache-plan-delete-7"]').trigger('click'); expect(api.deletePlan).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="confirm-delete"]').trigger('click'); await flushPromises(); expect(api.deletePlan).toHaveBeenCalledWith(7)
  })
  it('shows round metering and allows a sample to be withdrawn while keeping history', async () => {
    wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="cache-result-view-9"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('1000')
    await wrapper.get('[data-testid="cache-result-withdraw"]').trigger('click'); await flushPromises()
    expect(api.withdraw).toHaveBeenCalledWith(90)
    expect(wrapper.find('[data-testid="cache-result-withdraw"]').exists()).toBe(false)
  })
})
