import type { UsageLog } from '@/types'
import { BILLING_MODE_IMAGE, BILLING_MODE_VIDEO, getDisplayBillingMode } from './billingMode'

type UsageCacheRateRow = Partial<
  Pick<UsageLog, 'input_tokens' | 'cache_creation_tokens' | 'cache_read_tokens' | 'billing_mode' | 'image_count'>
>

/** Cache reads as a percentage of the input buckets stored in the usage log. */
export const usageCacheRate = (row: UsageCacheRateRow | null | undefined): number | null => {
  const billingMode = getDisplayBillingMode({ image_count: row?.image_count ?? 0, billing_mode: row?.billing_mode })
  if (billingMode === BILLING_MODE_IMAGE || billingMode === BILLING_MODE_VIDEO) return null

  const inputTokens = row?.input_tokens ?? NaN
  const cacheCreationTokens = row?.cache_creation_tokens ?? NaN
  const cacheReadTokens = row?.cache_read_tokens ?? NaN
  if ([inputTokens, cacheCreationTokens, cacheReadTokens].some((tokens) => !Number.isFinite(tokens) || tokens < 0)) {
    return null
  }

  // Cache creation TTL buckets are already included in cache_creation_tokens.
  const totalInputTokens = inputTokens + cacheCreationTokens + cacheReadTokens
  if (!Number.isFinite(totalInputTokens) || totalInputTokens <= 0) return null
  return (cacheReadTokens / totalInputTokens) * 100
}

export const formatUsageCacheRate = (row: UsageCacheRateRow | null | undefined): string | null => {
  const rate = usageCacheRate(row)
  return rate == null ? null : `${rate.toFixed(1)}%`
}
