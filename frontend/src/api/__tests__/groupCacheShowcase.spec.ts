import { describe, expect, it, vi } from 'vitest'
import { canPublishCacheRun, publishCacheRun } from '../groupCacheShowcase'
import { DEFAULT_CACHE_SETTINGS, type CacheTestRun } from '@/utils/groupCacheTest'
const mocks = vi.hoisted(() => ({ post: vi.fn().mockResolvedValue({ data: { id: 1 } }) }))
vi.mock('../client', () => ({ apiClient: { post: mocks.post } }))
describe('cache showcase publication payload', () => {
  it('sends only bill identities, never browser metrics or nested private fields', async () => {
    const run = { version: 1, id: 'demo', groupId: 7, keyId: 10, status: 'completed', settings: { ...DEFAULT_CACHE_SETTINGS, model: 'demo', rounds: 2 }, key: 'SECRET', rounds: [1, 2].map(index => ({ index, status: 'completed', billingState: 'matched', requestId: `client:${index}`, tokens: { read: 9999999 }, bill: { id: index, requestId: `client:${index}`, accountId: 55, actualCost: 999, private: 'SECRET' } })) } as unknown as CacheTestRun
    expect(canPublishCacheRun(run)).toBe(true)
    await publishCacheRun(run)
    expect(mocks.post).toHaveBeenCalledWith('/admin/group-cache-showcase/items', { run_id: 'demo', group_id: 7, api_key_id: 10, model: 'demo', protocol: 'responses', rounds: [{ usage_log_id: 1, request_id: 'client:1' }, { usage_log_id: 2, request_id: 'client:2' }] })
    expect(JSON.stringify(mocks.post.mock.calls)).not.toContain('SECRET')
    run.rounds[1].requestId = 'client:mismatch'
    await expect(publishCacheRun(run)).rejects.toThrow('not ready')
    expect(mocks.post).toHaveBeenCalledTimes(1)
  })
})
