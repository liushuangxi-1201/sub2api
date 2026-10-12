import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GroupCacheShowcaseView from '../GroupCacheShowcaseView.vue'
import type { CacheShowcaseItem, CacheShowcaseView } from '@/api/groupCacheShowcase'
const mocks = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/groupCacheShowcase', () => ({ getCacheShowcase: mocks.get }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const item: CacheShowcaseItem = { group_id: 7, group_name: 'Demo group', platform: 'openai', model: 'demo-model', protocol: 'responses', last_tested_at: '2026-10-12T08:00:00Z', samples: 4, latency_samples: 3, input_tokens: 1000, cache_read_tokens: 922, cache_hit_rate: 0.922, first_token_ms: 4700 }
const data = (items: CacheShowcaseItem[] = [item]): CacheShowcaseView => ({ enabled: true, window_start: '2026-10-11T08:00:00Z', window_end: '2026-10-12T08:00:00Z', items })
let wrapper: ReturnType<typeof mount>
beforeEach(() => { vi.clearAllMocks(); mocks.get.mockResolvedValue(data()) })
afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })
describe('GroupCacheShowcaseView', () => {
  it('shows the requested metrics and filters by group', async () => {
    mocks.get.mockResolvedValue(data([item, { ...item, group_id: 8, group_name: 'Other group' }]))
    wrapper = mount(GroupCacheShowcaseView); await flushPromises()
    expect(wrapper.findAll('[data-testid="cache-showcase-card"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="showcase-latency"]').text()).toBe('4.7s')
    expect(wrapper.get('[data-testid="showcase-rate"]').text()).toBe('92.2%')
    await wrapper.get('[data-testid="showcase-group"]').setValue('8')
    expect(wrapper.findAll('[data-testid="cache-showcase-card"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="cache-showcase-card"]').text()).toContain('Other group')
  })
  it('keeps missing metrics distinct from measured zero', async () => {
    mocks.get.mockResolvedValue(data([{ ...item, samples: 0, latency_samples: 0, cache_hit_rate: null, first_token_ms: null }, { ...item, group_id: 8, cache_hit_rate: 0, first_token_ms: 0 }]))
    wrapper = mount(GroupCacheShowcaseView); await flushPromises()
    expect(wrapper.findAll('[data-testid="showcase-rate"]').map(n => n.text())).toEqual(['—', '0.0%'])
    expect(wrapper.findAll('[data-testid="showcase-latency"]').map(n => n.text())).toEqual(['—', '0.0s'])
    expect(wrapper.findAll('[data-testid="showcase-no-samples"]')).toHaveLength(1)
  })
  it('removes displayed data when disabled or a refreshed request fails', async () => {
    wrapper = mount(GroupCacheShowcaseView); await flushPromises()
    mocks.get.mockResolvedValue({ ...data(), enabled: false, items: [] })
    await wrapper.get('[data-testid="showcase-refresh"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="cache-showcase-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="showcase-disabled"]').exists()).toBe(true)
    mocks.get.mockRejectedValue(new Error('offline'))
    await wrapper.get('[data-testid="showcase-refresh"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="showcase-disabled"]').exists()).toBe(false)
  })
  it('refreshes the rolling window every minute and aborts on unmount', async () => {
    vi.useFakeTimers()
    wrapper = mount(GroupCacheShowcaseView); await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(mocks.get).toHaveBeenCalledTimes(2)
    const signal = mocks.get.mock.calls[1][0] as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(mocks.get).toHaveBeenCalledTimes(2)
  })
})
