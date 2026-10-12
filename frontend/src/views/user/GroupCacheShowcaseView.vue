<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><h1 class="text-2xl font-semibold">{{ t('admin.groupCacheTest.showcase.userTitle') }}</h1><p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.groupCacheTest.showcase.userDescription') }}</p></div>
        <button class="btn btn-secondary" :disabled="loading" data-testid="showcase-refresh" @click="load">{{ t('admin.groupCacheTest.reload') }}</button>
      </header>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ t('admin.groupCacheTest.showcase.loadError') }}</p>
      <p v-if="loading && !view" role="status" class="py-12 text-center text-gray-500">{{ t('admin.groupCacheTest.loading') }}</p>
      <section v-else-if="view && !view.enabled" class="showcase-empty" data-testid="showcase-disabled">{{ t('admin.groupCacheTest.showcase.disabled') }}</section>
      <template v-else-if="view?.enabled">
        <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-gray-500">
          <p>{{ t('admin.groupCacheTest.showcase.window', { start: date(view.window_start), end: date(view.window_end) }) }}</p>
          <label class="flex items-center gap-2"><span>{{ t('admin.groupCacheTest.group') }}</span><select v-model="filter" class="input !w-auto" data-testid="showcase-group"><option value="">{{ t('admin.groupCacheTest.showcase.allGroups') }}</option><option v-for="group in groups" :key="group.id" :value="String(group.id)">{{ group.name }}</option></select></label>
        </div>
        <section v-if="!visibleItems.length" class="showcase-empty" data-testid="showcase-empty">{{ t('admin.groupCacheTest.showcase.empty') }}</section>
        <div v-else class="grid items-start gap-5 lg:grid-cols-2 2xl:grid-cols-3">
          <article v-for="item in visibleItems" :key="`${item.group_id}:${item.model}:${item.protocol}`" class="min-w-0 rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" data-testid="cache-showcase-card">
            <header><h2 class="break-words text-lg font-semibold">{{ item.group_name }}</h2><p class="mt-1 break-all text-sm text-gray-500">{{ item.model }} · {{ protocolName(item.protocol) }}</p></header>
            <dl class="my-5 grid grid-cols-2 gap-4">
              <div class="showcase-metric"><dt><Icon name="bolt" size="md" aria-hidden="true" /><span>{{ t('admin.groupCacheTest.showcase.firstToken') }}</span></dt><dd data-testid="showcase-latency">{{ item.first_token_ms == null ? '—' : (item.first_token_ms / 1000).toFixed(1) }}<small v-if="item.first_token_ms != null">s</small></dd></div>
              <div class="showcase-metric"><dt><Icon name="globe" size="md" aria-hidden="true" /><span>{{ t('admin.groupCacheTest.showcase.cacheRate') }}</span></dt><dd data-testid="showcase-rate">{{ item.cache_hit_rate == null ? '—' : (item.cache_hit_rate * 100).toFixed(1) }}<small v-if="item.cache_hit_rate != null">%</small></dd></div>
            </dl>
            <div class="space-y-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
              <p v-if="item.samples">{{ t('admin.groupCacheTest.showcase.sampleCount', { count: item.samples, latency: item.latency_samples }) }}</p>
              <p v-else class="text-amber-600 dark:text-amber-400" data-testid="showcase-no-samples">{{ t('admin.groupCacheTest.showcase.noRecentSamples') }}</p>
              <p>{{ t('admin.groupCacheTest.showcase.testedAt', { time: date(item.last_tested_at) }) }}</p>
            </div>
          </article>
        </div>
        <p class="max-w-5xl text-xs leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.groupCacheTest.showcase.method') }}</p>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { getCacheShowcase, type CacheShowcaseView } from '@/api/groupCacheShowcase'
const { t } = useI18n()
const view = ref<CacheShowcaseView | null>(null), filter = ref(''), loading = ref(false), error = ref(false)
let alive = true, controller: AbortController | null = null, timer: ReturnType<typeof setInterval> | undefined
const groups = computed(() => Array.from(new Map((view.value?.items ?? []).map(item => [item.group_id, { id: item.group_id, name: item.group_name }])).values()))
const visibleItems = computed(() => (view.value?.items ?? []).filter(item => !filter.value || String(item.group_id) === filter.value))
const date = (value: string) => new Date(value).toLocaleString()
const protocolName = (value: string) => ({ responses: 'Responses', chat: 'Chat Completions', messages: 'Messages' })[value] ?? value
async function load() {
  controller?.abort()
  const current = new AbortController(); controller = current
  loading.value = true; error.value = false
  try {
    const result = await getCacheShowcase(current.signal)
    if (alive && !current.signal.aborted) {
      view.value = result
      if (!result.items.some(item => String(item.group_id) === filter.value)) filter.value = ''
    }
  } catch { if (alive && !current.signal.aborted) { view.value = null; error.value = true } }
  finally { if (alive && !current.signal.aborted) loading.value = false }
}
function refreshVisible() { if (!document.hidden) void load() }
onMounted(() => { void load(); timer = setInterval(refreshVisible, 60_000); document.addEventListener('visibilitychange', refreshVisible) })
onBeforeUnmount(() => { alive = false; controller?.abort(); clearInterval(timer); document.removeEventListener('visibilitychange', refreshVisible) })
</script>

<style scoped>
.showcase-empty { @apply rounded-2xl border border-gray-200 bg-white px-6 py-16 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800; }
.showcase-metric { @apply flex min-w-0 flex-col rounded-3xl border-2 border-gray-200 bg-gray-50 px-4 py-5 dark:border-dark-600 dark:bg-dark-900 sm:px-5; }
.showcase-metric dt { @apply flex items-center gap-2 text-sm font-semibold tracking-wide text-gray-500 dark:text-gray-400; }
.showcase-metric dt svg { @apply shrink-0; }
.showcase-metric dd { @apply mt-auto whitespace-nowrap pt-4 font-mono text-3xl font-bold leading-none tracking-wide text-gray-900 dark:text-gray-100; }
.showcase-metric small { @apply ml-1 text-lg; }
</style>
