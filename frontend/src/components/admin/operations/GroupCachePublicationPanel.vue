<template>
  <section class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" data-testid="cache-publication-panel">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div class="max-w-3xl"><h2 class="font-semibold">{{ t('admin.groupCacheTest.showcase.title') }}</h2><p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.groupCacheTest.showcase.hint') }}</p></div>
      <RouterLink to="/group-cache-showcase" class="btn btn-secondary btn-sm">{{ t('admin.groupCacheTest.showcase.view') }}</RouterLink>
    </div>
    <p v-if="error" role="alert" class="mt-3 text-sm text-red-600">{{ error }}</p>
    <p v-if="notice" role="status" class="mt-3 text-sm text-primary-600">{{ notice }}</p>
    <div class="mt-4 flex flex-wrap items-center justify-between gap-4 rounded-lg bg-gray-50 p-4 dark:bg-dark-900">
      <div class="flex items-center gap-3">
        <Toggle :model-value="enabled" :disabled="settingsLoading || saving || settingsFailed" :aria-label="t('admin.groupCacheTest.showcase.enabled')" data-testid="cache-showcase-toggle" @update:model-value="changeEnabled" />
        <span class="text-sm font-medium">{{ t('admin.groupCacheTest.showcase.enabled') }}</span>
        <button v-if="settingsFailed" class="text-sm underline" @click="loadSettings">{{ t('admin.groupCacheTest.reload') }}</button>
      </div>
      <button class="btn btn-primary btn-sm" :disabled="busy || saving || listLoading || listFailed || !publishable || alreadyPublished" data-testid="cache-publish" @click="publish">{{ t(alreadyPublished ? 'admin.groupCacheTest.showcase.published' : 'admin.groupCacheTest.showcase.publish') }}</button>
    </div>
    <p class="mt-2 text-xs leading-5 text-gray-500">{{ t(!enabled ? 'admin.groupCacheTest.showcase.hiddenHint' : 'admin.groupCacheTest.showcase.visibleHint') }} {{ t('admin.groupCacheTest.showcase.requirement') }}</p>
    <details class="mt-4" data-testid="cache-publications">
      <summary class="cursor-pointer text-sm font-medium">{{ t('admin.groupCacheTest.showcase.manage', { count: items.length }) }}</summary>
      <p v-if="!targetGroup" class="mt-3 text-sm text-gray-500">{{ t('admin.groupCacheTest.selectGroup') }}</p>
      <p v-else-if="listLoading" role="status" class="mt-3 text-sm text-gray-500">{{ t('admin.groupCacheTest.loading') }}</p>
      <button v-else-if="listFailed" class="mt-3 text-sm underline" @click="loadItems">{{ t('admin.groupCacheTest.reload') }}</button>
      <p v-else-if="!items.length" class="mt-3 text-sm text-gray-500">{{ t('admin.groupCacheTest.showcase.noPublications') }}</p>
      <ul v-else class="mt-2 divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="item in items" :key="item.id" class="flex flex-wrap items-center gap-3 py-3 text-sm">
          <span class="font-medium">{{ item.group_name }} · {{ item.model }}</span><span class="text-xs text-gray-500">{{ new Date(item.tested_at).toLocaleString() }}</span>
          <button class="ml-auto shrink-0 text-red-600 underline" :disabled="saving" :data-testid="`cache-withdraw-${item.id}`" @click="withdraw(item)">{{ t('admin.groupCacheTest.showcase.withdraw') }}</button>
        </li>
      </ul>
    </details>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { canPublishCacheRun, getCacheShowcaseSettings, listCachePublications, publishCacheRun, removeCachePublication, setCacheShowcaseEnabled, type CachePublication } from '@/api/groupCacheShowcase'
import type { CacheTestRun } from '@/utils/groupCacheTest'

const props = defineProps<{ run: CacheTestRun | null; groupId: number | null; busy: boolean }>()
const { t } = useI18n(), app = useAppStore(), auth = useAuthStore()
const enabled = ref(false), settingsLoading = ref(true), settingsFailed = ref(false), saving = ref(false)
const listLoading = ref(false), listFailed = ref(false), items = ref<CachePublication[]>([])
const error = ref(''), notice = ref('')
let alive = true, listController: AbortController | null = null
const settingsController = new AbortController()
const targetGroup = computed(() => props.run?.groupId ?? props.groupId)
const publishable = computed(() => canPublishCacheRun(props.run))
const alreadyPublished = computed(() => items.value.some(item => item.run_id === props.run?.id && item.created_by === auth.user?.id))
async function loadSettings() {
  settingsLoading.value = true; settingsFailed.value = false; error.value = ''
  try { const result = await getCacheShowcaseSettings(settingsController.signal); if (alive) enabled.value = result.enabled }
  catch { if (alive) { settingsFailed.value = true; error.value = t('admin.groupCacheTest.showcase.loadError') } }
  finally { if (alive) settingsLoading.value = false }
}
async function loadItems() {
  listController?.abort()
  const controller = new AbortController(); listController = controller
  items.value = []; listFailed.value = false
  const groupId = targetGroup.value
  if (!groupId) { listLoading.value = false; return }
  listLoading.value = true
  try { const result = await listCachePublications(groupId, controller.signal); if (alive && !controller.signal.aborted) items.value = result }
  catch { if (alive && !controller.signal.aborted) { listFailed.value = true; error.value = t('admin.groupCacheTest.showcase.loadError') } }
  finally { if (alive && !controller.signal.aborted) listLoading.value = false }
}
async function changeEnabled(value: boolean) {
  if (saving.value || settingsLoading.value || settingsFailed.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    const result = await setCacheShowcaseEnabled(value)
    if (alive) { enabled.value = result.enabled; notice.value = t('admin.groupCacheTest.showcase.saved') }
    await app.fetchPublicSettings(true)
  } catch { if (alive) error.value = t('admin.groupCacheTest.showcase.saveError') }
  finally { if (alive) saving.value = false }
}
async function publish() {
  const run = props.run
  if (!canPublishCacheRun(run) || props.busy || saving.value || listLoading.value || listFailed.value || alreadyPublished.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    await publishCacheRun(run)
    if (alive) { notice.value = t('admin.groupCacheTest.showcase.published'); await loadItems() }
  } catch { if (alive) error.value = t('admin.groupCacheTest.showcase.publishError') }
  finally { if (alive) saving.value = false }
}
async function withdraw(item: CachePublication) {
  if (saving.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    await removeCachePublication(item.id)
    if (alive) { notice.value = t('admin.groupCacheTest.showcase.withdrawn'); await loadItems() }
  } catch { if (alive) error.value = t('admin.groupCacheTest.showcase.saveError') }
  finally { if (alive) saving.value = false }
}
watch(targetGroup, () => { notice.value = ''; void loadItems() }, { immediate: true })
onMounted(loadSettings)
onBeforeUnmount(() => { alive = false; settingsController.abort(); listController?.abort() })
</script>
