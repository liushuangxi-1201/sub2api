<template>
  <AppLayout>
    <div class="cache-tests">
      <SmartOpsNav />
      <header class="ops-heading">
        <div>
          <p class="eyebrow">{{ t('accountOps.smartTitle') }}</p>
          <h2>{{ t('admin.groupCachePlans.title') }}</h2>
          <p class="subtitle">{{ t('admin.groupCachePlans.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <RouterLink to="/group-cache-showcase" class="btn btn-secondary inline-flex items-center gap-2">
            <Icon name="eye" size="sm" />{{ t('admin.groupCachePlans.viewShowcase') }}
          </RouterLink>
          <button class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading" @click="load">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />{{ t('admin.groupCachePlans.refresh') }}
          </button>
          <button class="btn btn-primary inline-flex items-center gap-2" data-testid="cache-tests-create" @click="openEditor()">
            <Icon name="plus" size="sm" />{{ t('admin.groupCachePlans.create') }}
          </button>
        </div>
      </header>

      <p v-if="error" role="alert" class="error-banner">{{ error }}</p>
      <p v-if="notice" role="status" class="success-banner">{{ notice }}</p>

      <div class="ops-columns">
        <section class="settings-card" data-testid="cache-showcase-settings">
          <div class="section-title">
            <span class="icon-tile"><Icon name="eye" size="md" /></span>
            <div><h3>{{ t('admin.groupCachePlans.showcase.title') }}</h3><p>{{ t('admin.groupCachePlans.showcase.hint') }}</p></div>
          </div>
          <div v-if="!draft" class="space-y-4 p-5" role="status">
            <span v-for="n in 4" :key="n" class="block h-8 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" />
          </div>
          <form v-else class="settings-form" @submit.prevent="saveSettings">
            <fieldset :disabled="savingSettings">
              <div class="enable-row">
                <span><strong>{{ t('admin.groupCachePlans.showcase.enabled') }}</strong><small>{{ t('admin.groupCachePlans.showcase.enabledHint') }}</small></span>
                <Toggle v-model="draft.enabled" :aria-label="t('admin.groupCachePlans.showcase.enabled')" data-testid="cache-showcase-enabled" />
              </div>
              <div class="enable-row mt-5">
                <span><strong>{{ t('admin.groupCachePlans.showcase.apiEnabled') }}</strong><small>{{ t('admin.groupCachePlans.showcase.apiEnabledHint') }}</small></span>
                <Toggle v-model="draft.api_enabled" :aria-label="t('admin.groupCachePlans.showcase.apiEnabled')" data-testid="cache-showcase-api-enabled" />
              </div>
              <p v-if="!draft.enabled" class="field-hint text-amber-600 dark:text-amber-400" data-testid="cache-showcase-api-requires-gallery">
                {{ t('admin.groupCachePlans.showcase.apiRequiresGallery') }}
              </p>
              <button type="button" class="mt-2 inline-flex items-center gap-1.5 text-xs text-primary-600 hover:underline dark:text-primary-400" data-testid="cache-showcase-api-info" @click="showApiInfo = true">
                <Icon name="link" size="xs" />{{ t('admin.groupCachePlans.showcase.apiInfo') }}
              </button>
              <label class="field-label" for="cache-showcase-max-items">{{ t('admin.groupCachePlans.showcase.maxItems') }}</label>
              <div class="relative">
                <input id="cache-showcase-max-items" v-model.number="draft.max_items" type="number" min="1" max="100" required class="input w-full pr-12" data-testid="cache-showcase-max-items" />
                <span class="unit">{{ t('admin.groupCachePlans.showcase.maxItemsUnit') }}</span>
              </div>
              <p class="field-hint">{{ t('admin.groupCachePlans.showcase.maxItemsHint') }}</p>
              <div class="enable-row mt-5 mb-0">
                <span><strong>{{ t('admin.groupCachePlans.showcase.autoCleanup') }}</strong><small>{{ t('admin.groupCachePlans.showcase.autoCleanupHint') }}</small></span>
                <Toggle v-model="draft.auto_cleanup" :aria-label="t('admin.groupCachePlans.showcase.autoCleanup')" data-testid="cache-showcase-auto-cleanup" />
              </div>
              <template v-if="draft.auto_cleanup">
                <label class="field-label" for="cache-showcase-retention">{{ t('admin.groupCachePlans.showcase.retentionDays') }}</label>
                <div class="relative">
                  <input id="cache-showcase-retention" v-model.number="draft.retention_days" type="number" min="1" max="90" required class="input w-full pr-12" data-testid="cache-showcase-retention-days" />
                  <span class="unit">{{ t('admin.groupCachePlans.showcase.retentionDaysUnit') }}</span>
                </div>
                <p class="field-hint">{{ t('admin.groupCachePlans.showcase.retentionDaysHint') }}</p>
              </template>
              <div class="settings-actions">
                <span>{{ settingsDirty ? t('admin.groupCachePlans.showcase.unsaved') : t('admin.groupCachePlans.showcase.savedState') }}</span>
                <button class="btn btn-primary" :disabled="savingSettings || !settingsDirty" data-testid="cache-showcase-save">
                  {{ t(savingSettings ? 'admin.groupCachePlans.showcase.saving' : 'admin.groupCachePlans.showcase.save') }}
                </button>
              </div>
            </fieldset>
          </form>
        </section>

        <section class="plans-card" data-testid="cache-group-tests">
          <header class="panel-heading">
            <div><h3>{{ t('admin.groupCachePlans.plans.title') }}<span class="count">{{ plans.length }}</span></h3><p>{{ t('admin.groupCachePlans.plans.hint') }}</p></div>
          </header>
          <div v-if="!plansLoaded" class="space-y-3 p-5" role="status">
            <span v-for="n in 3" :key="n" class="block h-24 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800" />
          </div>
          <div v-else-if="!plans.length" class="empty-state">
            <Icon name="beaker" size="xl" />
            <p>{{ t('admin.groupCachePlans.plans.empty') }}</p>
            <button class="btn btn-secondary" @click="openEditor()">{{ t('admin.groupCachePlans.create') }}</button>
          </div>
          <ul v-else class="plan-list">
            <li v-for="plan in plans" :key="plan.id" class="plan-row" :data-testid="`cache-plan-${plan.id}`">
              <div class="plan-main">
                <span class="group-icon" :class="platformBadgeLightClass(plan.group_platform)">
                  <PlatformIcon :platform="plan.group_platform as GroupPlatform" size="sm" />
                </span>
                <div class="min-w-0">
                  <p class="plan-title">
                    <strong :title="plan.group_name">{{ plan.group_name }}</strong>
                    <span class="inline-flex flex-wrap items-center gap-x-2 text-xs font-normal tabular-nums text-gray-500 dark:text-gray-400" :title="t('admin.groupCachePlans.cost.hint')" :data-testid="`cache-plan-cost-${plan.id}`">
                      <span class="whitespace-nowrap">{{ t('admin.groupCachePlans.cost.today', { amount: costLabel(plan.today_cost_usd, plan.today_cost_incomplete) }) }}</span>
                      <span aria-hidden="true">·</span>
                      <span class="whitespace-nowrap">{{ t('admin.groupCachePlans.cost.total', { amount: costLabel(plan.total_cost_usd, plan.total_cost_incomplete) }) }}</span>
                    </span>
                    <span class="state-chip" :class="plan.enabled ? 'state-on' : 'state-off'">{{ t(plan.enabled ? 'admin.groupCachePlans.plans.active' : 'admin.groupCachePlans.plans.paused') }}</span>
                    <span v-if="isRunning(plan)" class="state-chip state-running">{{ t('admin.groupCachePlans.plans.running') }}</span>
                  </p>
                  <p class="plan-meta">
                    <code v-if="plan.model_id">{{ plan.model_id }}</code>
                    <span v-else class="text-amber-600 dark:text-amber-400">{{ t('admin.groupCachePlans.plans.modelMissing') }}</span>
                    <span>·</span><span>{{ effortLabel(plan.config?.reasoning_effort) }}</span>
                    <span>·</span><span>{{ t('admin.groupCachePlans.plans.parallel', { count: plan.config?.parallel_count || 1 }) }}</span>
                    <span>·</span><span>{{ scheduleLabel(plan.cron_expression) }}</span>
                  </p>
                  <p v-if="plan.group_status !== 'active'" class="plan-warning">
                    {{ t(plan.group_status === 'deleted' ? 'admin.groupCachePlans.plans.groupDeleted' : 'admin.groupCachePlans.plans.groupDisabled') }}
                  </p>
                  <p class="plan-last" :data-testid="`cache-plan-last-${plan.id}`">
                    <span class="text-gray-400">{{ t('admin.groupCachePlans.plans.lastRun') }}</span>
                    <template v-if="plan.last_result">
                      <span class="result-dot" :class="plan.last_result.status === 'success' ? 'dot-ok' : 'dot-bad'" />
                      <span>{{ formatDateTimeToMinute(plan.last_result.started_at) }}</span>
                      <span v-if="plan.last_result.account_id">{{ t('admin.groupCachePlans.plans.routedTo', { name: accountName(plan.last_result) }) }}</span>
                      <span v-else>{{ t('admin.groupCachePlans.plans.noAccount') }}</span>
                      <span v-if="plan.last_result.attempts.length" class="text-amber-600 dark:text-amber-400">{{ t('admin.groupCachePlans.plans.failedOver', { count: plan.last_result.attempts.length }) }}</span>
                      <span v-if="plan.last_result.status !== 'success'" class="truncate text-red-500" :title="resultError(plan.last_result.error_message)">{{ resultError(plan.last_result.error_message) }}</span>
                    </template>
                    <span v-else>{{ t('admin.groupCachePlans.plans.never') }}</span>
                  </p>
                  <p v-if="plan.enabled && plan.next_run_at" class="plan-next">
                    {{ t('admin.groupCachePlans.plans.nextRun') }} {{ formatDateTimeToMinute(plan.next_run_at) }}
                  </p>
                </div>
              </div>
              <div class="plan-actions">
                <button :disabled="!!pending[plan.id] || isRunning(plan) || !plan.model_id" :data-testid="`cache-plan-run-${plan.id}`" @click="runNow(plan)">
                  <Icon name="play" size="xs" />{{ t('admin.groupCachePlans.plans.runNow') }}
                </button>
                <button :disabled="!!pending[plan.id]" :data-testid="`cache-plan-toggle-${plan.id}`" @click="toggle(plan)">
                  {{ t(plan.enabled ? 'admin.groupCachePlans.plans.pause' : 'admin.groupCachePlans.plans.enable') }}
                </button>
                <button :disabled="!!pending[plan.id]" :data-testid="`cache-plan-edit-${plan.id}`" @click="openEditor(plan)">
                  <Icon name="edit" size="xs" />{{ t('admin.groupCachePlans.plans.edit') }}
                </button>
                <button class="danger" :disabled="!!pending[plan.id]" :data-testid="`cache-plan-delete-${plan.id}`" @click="deleting = plan">
                  <Icon name="trash" size="xs" />{{ t('admin.groupCachePlans.plans.delete') }}
                </button>
              </div>
            </li>
          </ul>
        </section>
      </div>

      <section class="history-card" data-testid="cache-test-history">
        <header class="panel-heading">
          <div><h3>{{ t('admin.groupCachePlans.history.title') }}</h3><p>{{ t('admin.groupCachePlans.history.hint') }}</p></div>
        </header>
        <div class="history-scroll">
          <table>
            <thead>
              <tr>
                <th>{{ t('admin.groupCachePlans.history.time') }}</th>
                <th>{{ t('admin.groupCachePlans.history.group') }}</th>
                <th>{{ t('admin.groupCachePlans.history.model') }}</th>
                <th>{{ t('admin.groupCachePlans.history.account') }}</th>
                <th>{{ t('admin.groupCachePlans.history.result') }}</th>
                <th>{{ t('admin.groupCachePlans.history.duration') }}</th>
                <th>{{ t('admin.groupCachePlans.cost.column') }}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr v-if="resultsLoading && !results.length">
                <td colspan="8" class="text-center" role="status">{{ t('admin.groupCachePlans.history.loading') }}</td>
              </tr>
              <tr v-for="result in results" :key="result.id" :data-testid="`cache-result-${result.id}`">
                <td class="whitespace-nowrap">{{ formatDateTimeToMinute(result.started_at) }}</td>
                <td><strong :title="result.group_name">{{ result.group_name }}</strong></td>
                <td><code>{{ result.model_id || '—' }}</code></td>
                <td>
                  <strong v-if="result.account_id" :title="accountName(result)">{{ accountName(result) }}</strong>
                  <span v-else class="text-gray-400">{{ t('admin.groupCachePlans.plans.noAccount') }}</span>
                  <small v-if="result.attempts.length" class="text-amber-600 dark:text-amber-400">
                    {{ t('admin.groupCachePlans.history.tried', { accounts: result.attempts.map(attemptLabel).join('、') }) }}
                  </small>
                </td>
                <td>
                  <span class="result-badge" :class="result.status === 'success' ? 'badge-ok' : 'badge-bad'">
                    {{ t(result.status === 'success' ? 'admin.groupCachePlans.history.success' : 'admin.groupCachePlans.history.failed') }}
                  </span>
                  <small v-if="result.status !== 'success'" class="error-text" :title="resultError(result.error_message)">{{ resultError(result.error_message) }}</small>
                </td>
                <td class="whitespace-nowrap tabular-nums">{{ pelicanDurationLabel(t, result.latency_ms) }}</td>
                <td class="whitespace-nowrap tabular-nums" :title="t('admin.groupCachePlans.cost.hint')" :data-testid="`cache-result-cost-${result.id}`">{{ costLabel(result.cost_usd, result.cost_incomplete) }}</td>
                <td class="whitespace-nowrap text-right">
                  <button class="link-button" :data-testid="`cache-result-view-${result.id}`" @click="openPreview(result)">
                    {{ t('admin.groupCachePlans.history.view') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="resultsLoaded && !resultsLoading && !results.length" class="empty-state"><Icon name="document" size="xl" /><p>{{ t('admin.groupCachePlans.history.empty') }}</p></div>
        </div>
        <Pagination
          v-if="resultsTotal > 0"
          :total="resultsTotal"
          :page="resultsPage"
          :page-size="resultsPageSize"
          show-jump
          data-testid="cache-history-pagination"
          @update:page="changeResultsPage"
          @update:page-size="changeResultsPageSize"
        />
      </section>

      <aside class="scope-note"><Icon name="infoCircle" size="sm" /><p>{{ t('admin.groupCachePlans.accountNote') }} <RouterLink to="/admin/group-cache-probes" class="link-button">{{ t('admin.groupCachePlans.manualProbe') }}</RouterLink></p></aside>
    </div>

    <BaseDialog
      :show="editor !== null"
      :title="t(editor?.id ? 'admin.groupCachePlans.editor.editTitle' : 'admin.groupCachePlans.editor.createTitle')"
      width="normal"
      @close="editor = null"
    >
      <form v-if="editor" id="cache-test-editor" class="space-y-4" data-testid="cache-test-editor" @submit.prevent="savePlan">
        <div>
          <label class="field-label mt-0">{{ t('admin.groupCachePlans.editor.group') }}</label>
          <Select
            v-model="editor.form.group_id"
            :options="groupOptions"
            :disabled="!!editor.id"
            searchable
            :placeholder="t('admin.groupCachePlans.editor.groupPlaceholder')"
            :aria-label="t('admin.groupCachePlans.editor.group')"
          />
          <p v-if="editor.id" class="field-hint">{{ t('admin.groupCachePlans.editor.groupLocked') }}</p>
          <p v-else-if="groupsLoadFailed" class="field-hint text-amber-600 dark:text-amber-400">{{ t('admin.groupCachePlans.editor.groupsLoadFailed') }}</p>
        </div>
        <div>
          <label class="field-label mt-0" for="cache-test-model">{{ t('admin.groupCachePlans.editor.model') }}</label>
          <input id="cache-test-model" v-model.trim="editor.form.model_id" maxlength="100" class="input w-full" :placeholder="t('admin.groupCachePlans.editor.modelPlaceholder')" data-testid="cache-test-model" />
          <p class="field-hint">{{ t('admin.groupCachePlans.editor.modelHint') }}</p>
        </div>
        <div>
          <div class="flex items-center justify-between">
            <label class="field-label mt-0" for="cache-test-prompt">{{ t('admin.groupCachePlans.editor.prompt') }}</label>
            <button type="button" class="link-button text-xs" @click="editor.form.prompt = CACHE_TEST_PROMPT">{{ t('admin.groupCachePlans.editor.resetPrompt') }}</button>
          </div>
          <textarea id="cache-test-prompt" v-model="editor.form.prompt" rows="3" maxlength="32000" class="input w-full" data-testid="cache-test-prompt" />
          <p class="field-hint">{{ t('admin.groupCachePlans.editor.promptHint') }}</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="field-label mt-0">{{ t('admin.groupCachePlans.editor.effort') }}</label>
            <Select v-model="editor.form.reasoning_effort" :options="effortOptions" :aria-label="t('admin.groupCachePlans.editor.effort')" />
          </div>
          <div>
            <label class="field-label mt-0" for="cache-test-parallel">{{ t('admin.groupCachePlans.editor.parallel') }}</label>
            <input id="cache-test-parallel" v-model.number="editor.form.parallel_count" type="number" min="1" max="8" class="input w-full" />
            <p class="field-hint">{{ t('admin.groupCachePlans.editor.parallelHint') }}</p>
          </div>
        </div>
        <details class="rounded-xl border border-gray-200 p-3 dark:border-dark-700" data-testid="cache-test-sampling">
          <summary class="cursor-pointer text-sm font-medium">{{ t('admin.groupCachePlans.editor.sampling') }}</summary>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <div><label class="field-label mt-0" for="cache-test-rounds">{{ t('admin.groupCachePlans.editor.rounds') }}</label><input id="cache-test-rounds" v-model.number="editor.form.rounds" type="number" min="2" max="6" required class="input w-full" /></div>
            <div><label class="field-label mt-0" for="cache-test-prefix">{{ t('admin.groupCachePlans.editor.prefix') }}</label><input id="cache-test-prefix" v-model.number="editor.form.prefix_blocks" type="number" min="8" max="128" required class="input w-full" /></div>
            <div><label class="field-label mt-0" for="cache-test-output">{{ t('admin.groupCachePlans.editor.output') }}</label><input id="cache-test-output" v-model.number="editor.form.max_output_tokens" type="number" min="16" max="8192" required class="input w-full" /></div>
            <div><label class="field-label mt-0" for="cache-test-interval">{{ t('admin.groupCachePlans.editor.interval') }}</label><input id="cache-test-interval" v-model.number="editor.form.interval_ms" type="number" min="0" max="30000" required class="input w-full" /></div>
          </div><p class="field-hint">{{ t('admin.groupCachePlans.editor.samplingHint') }}</p>
        </details>
        <div>
          <label class="field-label mt-0">{{ t('admin.groupCachePlans.editor.schedule') }}</label>
          <Select v-model="editor.schedule" :options="scheduleOptions" :aria-label="t('admin.groupCachePlans.editor.schedule')" data-testid="cache-test-schedule" />
          <template v-if="editor.schedule === 'custom'">
            <label class="field-label" for="cache-test-cron">{{ t('admin.groupCachePlans.editor.cron') }}</label>
            <input id="cache-test-cron" v-model.trim="editor.form.cron_expression" class="input w-full font-mono" placeholder="*/30 * * * *" data-testid="cache-test-cron" />
            <p class="field-hint">{{ t('admin.groupCachePlans.editor.cronHint') }}</p>
          </template>
        </div>
        <div class="enable-row mb-0">
          <span><strong>{{ t('admin.groupCachePlans.editor.enabled') }}</strong></span>
          <Toggle v-model="editor.form.enabled" :aria-label="t('admin.groupCachePlans.editor.enabled')" data-testid="cache-test-enabled" />
        </div>
      </form>
      <template #footer>
        <div class="flex w-full justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editor = null">{{ t('admin.groupCachePlans.editor.cancel') }}</button>
          <button type="submit" form="cache-test-editor" class="btn btn-primary" :disabled="savingPlan" data-testid="cache-test-save">
            {{ t('admin.groupCachePlans.editor.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="preview !== null" :title="previewTitle" width="wide" @close="preview = null">
      <div v-if="preview?.status === 'ready'" class="space-y-4">
        <p class="field-hint">{{ t('admin.groupCachePlans.history.detailsHint') }}</p>
        <div class="history-scroll"><table>
          <thead><tr><th>{{ t('admin.groupCacheTest.round') }}</th><th>{{ t('admin.groupCacheTest.input') }}</th><th>{{ t('admin.groupCacheTest.cacheRead') }}</th><th>{{ t('admin.groupCacheTest.cacheWrite') }}</th><th>{{ t('admin.groupCacheTest.output') }}</th><th>{{ t('admin.groupCacheTest.firstText') }}</th><th>{{ t('admin.groupCachePlans.history.result') }}</th></tr></thead>
          <tbody><tr v-for="round in preview.result.rounds" :key="round.index"><td>{{ round.index === 1 ? t('admin.groupCacheTest.baseline') : t('admin.groupCacheTest.repeat', { index: round.index - 1 }) }} · {{ round.protocol || '—' }}</td><td>{{ round.input_tokens ?? '—' }}</td><td>{{ round.cache_read_tokens ?? '—' }}</td><td>{{ round.cache_write_tokens ?? '—' }}</td><td>{{ round.output_tokens ?? '—' }}</td><td>{{ round.first_token_ms == null ? '—' : (round.first_token_ms / 1000).toFixed(2) + ' s' }}</td><td>{{ t(round.status === 'success' ? 'admin.groupCachePlans.history.success' : 'admin.groupCachePlans.history.failed') }}</td></tr></tbody>
        </table></div>
        <p v-if="preview.result.error_message" role="alert" class="text-sm text-red-500">{{ resultError(preview.result.error_message) }}</p>
        <button v-if="preview.result.publication_id" class="btn btn-secondary" :disabled="withdrawing" data-testid="cache-result-withdraw" @click="withdrawResult">{{ t('admin.groupCachePlans.history.withdraw') }}</button>
        <p v-else class="field-hint">{{ t('admin.groupCachePlans.history.unpublished') }}</p>
      </div>
      <p v-else class="field-hint">{{ t(preview?.status === 'loading' ? 'admin.groupCachePlans.history.previewLoading' : 'admin.groupCachePlans.history.previewLoadFailed') }}</p>
      <template #footer><button class="btn btn-secondary" @click="preview = null">{{ t('admin.groupCachePlans.history.close') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="showApiInfo" :title="t('admin.groupCachePlans.showcase.apiInfo')" @close="showApiInfo = false">
      <div class="space-y-4 text-sm">
        <p>{{ t(settings?.enabled && settings?.api_enabled ? 'admin.groupCachePlans.api.available' : 'admin.groupCachePlans.api.disabled') }}</p>
        <p class="field-hint">{{ t('admin.groupCachePlans.api.hint') }}</p>
        <pre class="overflow-x-auto rounded-xl bg-gray-100 p-4 text-xs dark:bg-dark-800"><code>{{ apiExample }}</code></pre>
      </div>
      <template #footer><button class="btn btn-secondary" @click="showApiInfo = false">{{ t('admin.groupCachePlans.history.close') }}</button></template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleting !== null"
      :title="t('admin.groupCachePlans.plans.deleteTitle')"
      :message="t('admin.groupCachePlans.plans.deleteConfirm')"
      :confirm-text="t('admin.groupCachePlans.plans.delete')"
      danger
      @confirm="removePlan"
      @cancel="deleting = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import SmartOpsNav from '@/components/admin/operations/SmartOpsNav.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { pelicanDurationLabel } from '@/components/user/pelican/pelicanShowcaseFormat'
import { adminAPI } from '@/api'
import {
  cacheTestsAPI,
  CACHE_TEST_PROMPT,
  type CacheTestPlan,
  type CachePlanInput,
  type CachePlanResult,
  type CacheShowcaseSettings,
} from '@/api/admin/groupCacheTests'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import type { AdminGroup, GroupPlatform } from '@/types'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'

const SCHEDULES = [
  { key: 'every15m', cron: '*/15 * * * *' },
  { key: 'every30m', cron: '*/30 * * * *' },
  { key: 'hourly', cron: '0 * * * *' },
  { key: 'every2h', cron: '0 */2 * * *' },
  { key: 'every6h', cron: '0 */6 * * *' },
  { key: 'daily', cron: '0 9 * * *' },
] as const
const EFFORTS = ['minimal', 'low', 'medium', 'high', 'xhigh']
const POLL_MS = 15000

const { t } = useI18n()
const appStore = useAppStore()

function resultError(code: string) {
  const keys: Record<string, string> = {
    group_unavailable: 'groupUnavailable', no_available_account: 'noAccount',
    cache_test_timeout: 'timeout', cache_usage_unavailable: 'usageUnavailable',
    cache_protocol_changed: 'protocolChanged',
  }
  return t(`admin.groupCachePlans.errors.${keys[code] || 'requestFailed'}`)
}

const plans = ref<CacheTestPlan[]>([])
const plansLoaded = ref(false)
const settings = ref<CacheShowcaseSettings | null>(null)
const draft = ref<CacheShowcaseSettings | null>(null)
const results = ref<CachePlanResult[]>([])
const resultsLoaded = ref(false)
const resultsPage = ref(1)
const resultsPageSize = ref(getPersistedPageSize())
const resultsTotal = ref(0)
const resultsLoading = ref(false)
const groups = ref<AdminGroup[]>([])
const groupsLoadFailed = ref(false)
const loading = ref(false)
const savingSettings = ref(false)
const showApiInfo = ref(false)
const withdrawing = ref(false)
const apiExample = `curl "${window.location.origin}/api/v1/public/group-cache-showcase" -H "Authorization: Bearer YOUR_API_KEY"`
const savingPlan = ref(false)
const error = ref('')
const notice = ref('')
const pending = reactive<Record<number, string>>({})
const editor = ref<{ id: number | null; groupName: string; form: CachePlanInput; schedule: string } | null>(null)
const deleting = ref<CacheTestPlan | null>(null)
const preview = ref<{ result: CachePlanResult; status: 'loading' | 'ready' | 'error' } | null>(null)
let alive = true
let timer: ReturnType<typeof setInterval> | undefined
let resultsController: AbortController | undefined
let resultsRequest = 0
let polling = false

const settingsDirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== JSON.stringify(settings.value))
const groupOptions = computed(() => {
  const options = groups.value.map((group) => ({ value: group.id, label: `${group.name} · ${platformLabel(group.platform)}` }))
  // An edited plan's group may be disabled or deleted; still show its name.
  const current = editor.value
  if (current?.id && !options.some((option) => option.value === current.form.group_id)) {
    options.unshift({ value: current.form.group_id, label: current.groupName })
  }
  return options
})
const effortOptions = computed(() => EFFORTS.map((value) => ({ value, label: effortLabel(value) })))
const scheduleOptions = computed(() => [
  ...SCHEDULES.map((schedule) => ({ value: schedule.cron, label: t(`admin.groupCachePlans.schedules.${schedule.key}`) })),
  { value: 'custom', label: t('admin.groupCachePlans.schedules.custom') },
])
const previewTitle = computed(() => (preview.value
  ? t('admin.groupCachePlans.history.previewTitle', { group: preview.value.result.group_name || '—', model: preview.value.result.model_id || '—' })
  : ''))

// A preset keeps the cron in step; "custom" lets the admin type one.
watch(() => editor.value?.schedule, (schedule) => {
  if (editor.value && schedule && schedule !== 'custom') editor.value.form.cron_expression = schedule
})

function effortLabel(effort?: string) {
  return effort && EFFORTS.includes(effort) ? t(`pelicanShowcase.efforts.${effort}`) : '—'
}

function scheduleLabel(cron: string) {
  const preset = SCHEDULES.find((schedule) => schedule.cron === cron)
  return preset ? t(`admin.groupCachePlans.schedules.${preset.key}`) : cron
}

function accountName(result: CachePlanResult) {
  return result.account_name || `#${result.account_id}`
}

function costLabel(value: number | null | undefined, incomplete = false) {
  if (value == null) return t('admin.groupCachePlans.cost.unknown')
  const amount = value > 0 && value < 0.000001 ? '<$0.000001' : `$${value.toFixed(6)}`
  return incomplete ? t('admin.groupCachePlans.cost.partial', { amount }) : amount
}

function attemptLabel(attempt: CachePlanResult['attempts'][number]) {
  return attempt.account_name || `#${attempt.account_id}`
}

function isRunning(plan: CacheTestPlan) {
  return !!plan.running_until && new Date(plan.running_until).getTime() > Date.now()
}

function fail(err: unknown, fallback: string) {
  error.value = extractApiErrorMessage(err, fallback)
  notice.value = ''
}

function succeed(message: string) {
  notice.value = message
  error.value = ''
}

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    const [planList, showcase] = await Promise.all([
      cacheTestsAPI.listPlans(),
      cacheTestsAPI.getShowcaseSettings(),
      loadResults(),
    ])
    if (!alive) return
    plans.value = planList
    plansLoaded.value = true
    const wasDirty = settingsDirty.value
    settings.value = showcase
    // Keep an admin's unsaved edits across a refresh.
    if (!draft.value || !wasDirty) draft.value = { ...showcase }
  } catch (err: unknown) {
    fail(err, t('admin.groupCachePlans.loadError'))
  } finally {
    loading.value = false
  }
}

// Refresh the current page in place so polling cannot grow the table indefinitely.
async function poll() {
  if (loading.value || polling || document.hidden) return
  polling = true
  try {
    const [planList] = await Promise.all([cacheTestsAPI.listPlans(), loadResults(true)])
    if (!alive) return
    plans.value = planList
  } catch {
    // The next tick or a manual refresh tries again.
  } finally {
    polling = false
  }
}

async function loadResults(silent = false) {
  if (silent && resultsLoading.value) return
  const request = ++resultsRequest
  resultsController?.abort()
  const controller = new AbortController()
  resultsController = controller
  const pageNumber = resultsPage.value
  const pageSize = resultsPageSize.value
  resultsLoading.value = true
  if (!silent) results.value = []
  try {
    let page = await cacheTestsAPI.listResults(pageNumber, pageSize, 0, controller.signal)
    if (!alive || request !== resultsRequest) return
    // Deleting a plan or retention cleanup can remove the last page.
    const lastPage = Math.max(1, Math.ceil(page.total / pageSize))
    if (pageNumber > lastPage) {
      resultsPage.value = lastPage
      page = await cacheTestsAPI.listResults(lastPage, pageSize, 0, controller.signal)
    }
    if (!alive || request !== resultsRequest) return
    results.value = page.items
    resultsTotal.value = page.total
    resultsLoaded.value = true
  } catch (err: unknown) {
    if (alive && request === resultsRequest && !controller.signal.aborted && !silent) {
      fail(err, t('admin.groupCachePlans.loadError'))
    }
  } finally {
    if (alive && request === resultsRequest) resultsLoading.value = false
  }
}

function changeResultsPage(page: number) {
  if (page === resultsPage.value) return
  resultsPage.value = page
  void loadResults()
}

function changeResultsPageSize(pageSize: number) {
  if (pageSize === resultsPageSize.value) return
  resultsPageSize.value = pageSize
  resultsPage.value = 1
  void loadResults()
}

async function saveSettings() {
  if (!draft.value || savingSettings.value) return
  savingSettings.value = true
  try {
    const saved = await cacheTestsAPI.updateShowcaseSettings({ ...draft.value })
    settings.value = saved
    draft.value = { ...saved }
    succeed(t('admin.groupCachePlans.showcase.saved'))
    // The sidebar entry for users follows the public flag.
    await appStore.fetchPublicSettings(true)
  } catch (err: unknown) {
    fail(err, t('admin.groupCachePlans.showcase.saveFailed'))
  } finally {
    savingSettings.value = false
  }
}

async function loadGroups() {
  if (groups.value.length) return
  try {
    const all = await adminAPI.groups.getAll()
    groups.value = all.filter((group) => group.status === 'active' && ['openai', 'anthropic', 'composite', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode-go'].includes(group.platform))
    groupsLoadFailed.value = false
  } catch {
    groupsLoadFailed.value = true
  }
}

function inputFrom(plan: CacheTestPlan): CachePlanInput {
  return {
    group_id: plan.group_id,
    model_id: plan.model_id,
    cron_expression: plan.cron_expression,
    enabled: plan.enabled,
    prompt: plan.config?.prompt || CACHE_TEST_PROMPT,
    reasoning_effort: plan.config?.reasoning_effort || 'minimal',
    parallel_count: plan.config?.parallel_count || 1,
    rounds: plan.config?.rounds || 3, prefix_blocks: plan.config?.prefix_blocks || 32,
    max_output_tokens: plan.config?.max_output_tokens || 256, interval_ms: plan.config?.interval_ms ?? 1000,
  }
}

function openEditor(plan?: CacheTestPlan) {
  const form: CachePlanInput = plan
    ? inputFrom(plan)
    : { group_id: 0, model_id: '', cron_expression: SCHEDULES[1].cron, enabled: true, prompt: CACHE_TEST_PROMPT, reasoning_effort: 'minimal', parallel_count: 1, rounds: 3, prefix_blocks: 32, max_output_tokens: 256, interval_ms: 1000 }
  const schedule = SCHEDULES.some((preset) => preset.cron === form.cron_expression) ? form.cron_expression : 'custom'
  editor.value = { id: plan?.id ?? null, groupName: plan?.group_name ?? '', form, schedule }
  void loadGroups()
}

async function savePlan() {
  const current = editor.value
  if (!current || savingPlan.value) return
  const form = current.form
  if (!form.group_id || !form.model_id.trim() || !form.prompt.trim() || !form.cron_expression.trim()) {
    fail(null, t('admin.groupCachePlans.editor.required'))
    return
  }
  savingPlan.value = true
  try {
    const input = { ...form, parallel_count: Number(form.parallel_count) || 1 }
    if (current.id) {
      await cacheTestsAPI.updatePlan(current.id, input)
      succeed(t('admin.groupCachePlans.editor.updated'))
    } else {
      await cacheTestsAPI.createPlan(input)
      succeed(t('admin.groupCachePlans.editor.created'))
    }
    editor.value = null
    plans.value = await cacheTestsAPI.listPlans()
  } catch (err: unknown) {
    fail(err, t('admin.groupCachePlans.editor.saveFailed'))
  } finally {
    savingPlan.value = false
  }
}

async function toggle(plan: CacheTestPlan) {
  // A plan carried over without a model needs one before it can run.
  if (!plan.enabled && !plan.model_id) {
    openEditor(plan)
    if (editor.value) editor.value.form.enabled = true
    return
  }
  pending[plan.id] = 'toggle'
  try {
    await cacheTestsAPI.updatePlan(plan.id, { ...inputFrom(plan), enabled: !plan.enabled })
    plans.value = await cacheTestsAPI.listPlans()
  } catch (err: unknown) {
    fail(err, t('admin.groupCachePlans.plans.actionFailed'))
  } finally {
    delete pending[plan.id]
  }
}

async function runNow(plan: CacheTestPlan) {
  pending[plan.id] = 'run'
  try {
    await cacheTestsAPI.runPlan(plan.id)
    succeed(t('admin.groupCachePlans.plans.runStarted'))
    plans.value = await cacheTestsAPI.listPlans()
  } catch (err: unknown) {
    if (extractApiErrorCode(err) === 'CACHE_TEST_RUNNING') fail(null, t('admin.groupCachePlans.plans.alreadyRunning'))
    else fail(err, t('admin.groupCachePlans.plans.actionFailed'))
  } finally {
    delete pending[plan.id]
  }
}

async function removePlan() {
  const plan = deleting.value
  deleting.value = null
  if (!plan) return
  pending[plan.id] = 'delete'
  try {
    await cacheTestsAPI.deletePlan(plan.id)
    plans.value = plans.value.filter((item) => item.id !== plan.id)
    succeed(t('admin.groupCachePlans.plans.deleted'))
    await loadResults()
  } catch (err: unknown) {
    fail(err, t('admin.groupCachePlans.plans.actionFailed'))
  } finally {
    delete pending[plan.id]
  }
}

async function openPreview(result: CachePlanResult) {
  preview.value = { result, status: 'loading' }
  try {
    const full = await cacheTestsAPI.getResult(result.id)
    if (preview.value?.result.id !== result.id) return
    preview.value = { result: full, status: 'ready' }
  } catch {
    if (preview.value?.result.id === result.id) preview.value = { result, status: 'error' }
  }
}

async function withdrawResult() {
  const current = preview.value
  const publication = current?.result.publication_id
  if (!current || !publication || withdrawing.value) return
  withdrawing.value = true
  try {
    await cacheTestsAPI.withdraw(publication)
    if (preview.value?.result.id === current.result.id) preview.value.result.publication_id = null
    succeed(t('admin.groupCachePlans.history.withdrawn'))
    await Promise.all([loadResults(), load()])
  } catch (err) { fail(err, t('admin.groupCachePlans.plans.actionFailed')) }
  finally { withdrawing.value = false }
}

onMounted(() => {
  void load()
  timer = setInterval(poll, POLL_MS)
})
onBeforeUnmount(() => {
  alive = false
  resultsController?.abort()
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.cache-tests { @apply w-full min-w-0 text-gray-900 dark:text-gray-100; }
.ops-heading { @apply mb-6 flex flex-wrap items-center justify-between gap-4; }
.eyebrow { @apply mb-1 text-[11px] font-semibold tracking-widest text-primary-600; }
.ops-heading h2 { @apply text-2xl font-semibold tracking-tight; }
.subtitle { @apply mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400; }
.ops-columns { display: grid; grid-template-columns: minmax(300px, .32fr) minmax(0, .68fr); gap: 20px; align-items: start; }
.settings-card, .plans-card, .history-card { @apply min-w-0 overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900; }
.history-card { @apply mt-5; }
.section-title { @apply flex items-center gap-3 border-b border-gray-100 p-5 dark:border-dark-700; }
.icon-tile { @apply flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-950/30; }
.section-title h3, .panel-heading h3 { @apply text-base font-semibold; }
.section-title p, .panel-heading p { @apply mt-1 text-xs leading-relaxed text-gray-400; }
.settings-form { @apply p-5; }
.enable-row { @apply mb-2 flex items-center justify-between gap-4; }
.enable-row strong { @apply block text-sm font-medium; }
.enable-row small { @apply mt-1 block text-xs leading-relaxed text-gray-400; }
.field-label { @apply mb-2 mt-5 block text-xs font-medium text-gray-600 dark:text-gray-300; }
.field-hint { @apply mt-2 text-xs leading-relaxed text-gray-400; }
.unit { @apply pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-400; }
.settings-actions { @apply mt-5 flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700; }
.settings-actions span { @apply text-xs text-gray-400; }
.panel-heading { @apply border-b border-gray-100 p-5 dark:border-dark-700; }
.panel-heading .count { @apply ml-2 rounded-md bg-gray-100 px-2 py-0.5 text-xs font-normal tabular-nums text-gray-500 dark:bg-dark-800; }
.plan-list { @apply divide-y divide-gray-100 dark:divide-dark-800; }
.plan-row { @apply flex flex-wrap items-start justify-between gap-4 px-5 py-4; }
.plan-main { @apply flex min-w-0 items-start gap-3; flex: 1 1 20rem; }
.group-icon { @apply grid h-9 w-9 shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10; }
.plan-title { @apply flex flex-wrap items-center gap-2; }
.plan-title strong { @apply truncate text-sm font-semibold; max-width: 260px; }
.plan-meta { @apply mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-gray-500 dark:text-gray-400; }
.plan-meta code, td code { @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-dark-800 dark:text-gray-300; }
.plan-warning { @apply mt-2 text-xs text-amber-600 dark:text-amber-400; }
.plan-last { @apply mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-600 dark:text-gray-300; }
.plan-last .truncate { max-width: 340px; }
.plan-next { @apply mt-1 text-xs text-gray-400; }
.state-chip { @apply rounded-full px-2 py-0.5 text-[11px] font-medium; }
.state-on { @apply bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300; }
.state-off { @apply bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400; }
.state-running { @apply bg-primary-50 text-primary-700 dark:bg-primary-950/30 dark:text-primary-300; }
.result-dot { @apply inline-block h-2 w-2 rounded-full; }
.dot-ok { @apply bg-emerald-500; }
.dot-bad { @apply bg-red-500; }
.plan-actions { @apply ml-auto flex shrink-0 flex-wrap items-center gap-1; }
.plan-actions button { @apply inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-800; }
.plan-actions button.danger { @apply text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30; }
.history-scroll { @apply overflow-x-auto; }
table { @apply w-full text-left text-xs; min-width: 760px; }
thead { @apply bg-gray-50/60 text-gray-400 dark:bg-dark-800/50; }
th { @apply whitespace-nowrap px-4 py-3 font-medium; }
td { @apply border-b border-gray-100 px-4 py-3 align-top dark:border-dark-800; }
td strong { @apply block truncate text-[13px] font-medium; max-width: 200px; }
td small { @apply mt-1 block text-[11px]; }
.error-text { @apply truncate text-red-500; max-width: 280px; }
.result-badge { @apply inline-block rounded-md px-2 py-0.5 text-[11px] font-medium; }
.badge-ok { @apply bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300; }
.badge-bad { @apply bg-red-50 text-red-600 dark:bg-red-950/30 dark:text-red-300; }
.link-button { @apply text-primary-600 hover:underline dark:text-primary-400; }
.empty-state { @apply flex min-h-48 flex-col items-center justify-center gap-3 p-6 text-center text-sm text-gray-400; }
.scope-note { @apply mt-5 flex items-start gap-2 text-xs leading-6 text-gray-400; }
.scope-note svg { @apply mt-1 shrink-0; }
.error-banner { @apply mb-4 rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300; }
.success-banner { @apply mb-4 rounded-xl bg-emerald-50 p-3 text-sm text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300; }
button:disabled { @apply cursor-not-allowed opacity-40; }
@media (max-width: 1100px) { .ops-columns { grid-template-columns: 1fr; } }
</style>
