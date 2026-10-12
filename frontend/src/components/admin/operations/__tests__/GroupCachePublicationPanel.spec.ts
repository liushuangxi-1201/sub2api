import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GroupCachePublicationPanel from '../GroupCachePublicationPanel.vue'
import { DEFAULT_CACHE_SETTINGS, type CacheTestRun } from '@/utils/groupCacheTest'
const mocks = vi.hoisted(() => ({ get: vi.fn(), set: vi.fn(), list: vi.fn(), publish: vi.fn(), remove: vi.fn(), settings: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: {} }))
vi.mock('@/api/groupCacheShowcase', async () => ({
  ...await vi.importActual<typeof import('@/api/groupCacheShowcase')>('@/api/groupCacheShowcase'),
  getCacheShowcaseSettings: mocks.get, setCacheShowcaseEnabled: mocks.set, listCachePublications: mocks.list,
  publishCacheRun: mocks.publish, removeCachePublication: mocks.remove
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1 } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: mocks.settings }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
function completedRun(): CacheTestRun {
  return { version: 1, id: 'run', startedAt: new Date().toISOString(), finishedAt: new Date().toISOString(), status: 'completed', groupId: 7, groupName: 'Demo', keyId: 10, keyName: 'Key', settings: { ...DEFAULT_CACHE_SETTINGS, model: 'demo' }, rounds: [1, 2, 3].map(index => ({ index, status: 'completed', requestId: `client:${index}`, billingState: 'matched', bill: { id: index, requestId: `client:${index}` } })) } as CacheTestRun
}
const published = { id: 44, run_id: 'run', created_by: 1, group_id: 7, group_name: 'Demo', model: 'demo', tested_at: '2026-10-12T08:00:00Z' }
let wrapper: ReturnType<typeof mount>
const mountPanel = (run: CacheTestRun | null = completedRun()) => mount(GroupCachePublicationPanel, { props: { run, groupId: 7, busy: false }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
beforeEach(() => { vi.clearAllMocks(); mocks.get.mockResolvedValue({ enabled: false }); mocks.set.mockImplementation(async enabled => ({ enabled })); mocks.list.mockResolvedValue([]); mocks.publish.mockResolvedValue(published); mocks.remove.mockResolvedValue(undefined) })
afterEach(() => wrapper?.unmount())
describe('GroupCachePublicationPanel', () => {
  it('does not enable or publish automatically; saves visibility explicitly', async () => {
    wrapper = mountPanel(); await flushPromises()
    expect(mocks.publish).not.toHaveBeenCalled(); expect(mocks.set).not.toHaveBeenCalled()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[role="switch"]').trigger('click'); await flushPromises()
    expect(mocks.set).toHaveBeenCalledWith(true)
    expect(mocks.settings).toHaveBeenCalledWith(true)
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
  })
  it('publishes the selected completed test and supports withdrawal', async () => {
    const run = completedRun(); wrapper = mountPanel(run); await flushPromises()
    mocks.list.mockResolvedValue([published])
    await wrapper.get('[data-testid="cache-publish"]').trigger('click'); await flushPromises()
    expect(mocks.publish).toHaveBeenCalledWith(run)
    expect(mocks.set).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="cache-publish"]').attributes('disabled')).toBeDefined()
    mocks.list.mockResolvedValue([])
    await wrapper.get('[data-testid="cache-withdraw-44"]').trigger('click'); await flushPromises()
    expect(mocks.remove).toHaveBeenCalledWith(44)
    expect(wrapper.find('[data-testid="cache-withdraw-44"]').exists()).toBe(false)
  })
  it('blocks publication with pending bills or incomplete rounds', async () => {
    const run = completedRun(); run.rounds[1].bill = null
    wrapper = mountPanel(run); await flushPromises()
    expect(wrapper.get('[data-testid="cache-publish"]').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ run: { ...completedRun(), status: 'failed' } })
    expect(wrapper.get('[data-testid="cache-publish"]').attributes('disabled')).toBeDefined()
    expect(mocks.publish).not.toHaveBeenCalled()
  })
  it('keeps the saved switch on failure and aborts stale group lists', async () => {
    wrapper = mountPanel(null); await flushPromises()
    mocks.set.mockRejectedValue(new Error('offline'))
    await wrapper.get('[role="switch"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[role="alert"]').text()).toContain('saveError')
    const previous = mocks.list.mock.calls[0][1] as AbortSignal
    await wrapper.setProps({ groupId: 8 }); await flushPromises()
    expect(previous.aborted).toBe(true)
    expect(mocks.list).toHaveBeenLastCalledWith(8, expect.any(AbortSignal))
  })
})
