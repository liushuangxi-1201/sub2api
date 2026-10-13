import { describe, expect, it } from 'vitest'

import { formatUsageCacheRate, usageCacheRate } from '../usageCacheRate'

const cachedRow = {
  input_tokens: 123,
  cache_creation_tokens: 0,
  cache_read_tokens: 877,
  output_tokens: 5_000,
}

describe('usageCacheRate', () => {
  it('uses the complete input buckets and excludes output', () => {
    expect(usageCacheRate(cachedRow)).toBe(87.7)
    expect(formatUsageCacheRate(cachedRow)).toBe('87.7%')
  })

  it('includes cache creation in the denominator without counting TTL buckets twice', () => {
    const row = {
      input_tokens: 100,
      cache_creation_tokens: 200,
      cache_read_tokens: 700,
      cache_creation_5m_tokens: 150,
      cache_creation_1h_tokens: 50,
    }
    expect(usageCacheRate(row)).toBe(70)
    expect(formatUsageCacheRate(row)).toBe('70.0%')
    expect(usageCacheRate({ ...row, cache_read_tokens: 0 })).toBe(0)
  })

  it('distinguishes zero reads, creation-only input, fully cached input and no input', () => {
    expect(formatUsageCacheRate({ ...cachedRow, cache_read_tokens: 0 })).toBe('0.0%')
    expect(formatUsageCacheRate({ input_tokens: 0, cache_creation_tokens: 1_000, cache_read_tokens: 0 })).toBe('0.0%')
    expect(formatUsageCacheRate({ input_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 1_000 })).toBe('100.0%')
    expect(formatUsageCacheRate({ input_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0 })).toBeNull()
  })

  it('keeps missing input buckets unknown', () => {
    expect(usageCacheRate(null)).toBeNull()
    expect(usageCacheRate(undefined)).toBeNull()
    expect(usageCacheRate({})).toBeNull()
    for (const field of ['input_tokens', 'cache_creation_tokens', 'cache_read_tokens'] as const) {
      expect(formatUsageCacheRate({ ...cachedRow, [field]: undefined })).toBeNull()
    }
  })

  it.each([-1, NaN, Infinity, -Infinity])('rejects invalid input buckets %s', (invalid) => {
    for (const field of ['input_tokens', 'cache_creation_tokens', 'cache_read_tokens'] as const) {
      expect(usageCacheRate({ ...cachedRow, [field]: invalid })).toBeNull()
    }
  })

  it('rejects an overflowing total', () => {
    expect(usageCacheRate({ input_tokens: Number.MAX_VALUE, cache_creation_tokens: Number.MAX_VALUE, cache_read_tokens: 1 })).toBeNull()
  })

  it('excludes image/video billing while retaining recorded tokens in token/per-request billing', () => {
    expect(usageCacheRate({ ...cachedRow, billing_mode: 'image' })).toBeNull()
    expect(usageCacheRate({ ...cachedRow, billing_mode: 'video' })).toBeNull()
    expect(usageCacheRate({ ...cachedRow, image_count: 1 })).toBeNull()
    expect(usageCacheRate({ ...cachedRow, image_count: 1, billing_mode: 'token' })).toBe(87.7)
    expect(usageCacheRate({ ...cachedRow, billing_mode: 'per_request' })).toBe(87.7)
  })
})
