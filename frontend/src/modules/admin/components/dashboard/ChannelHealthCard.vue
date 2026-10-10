<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, Loader2, Play, Zap } from 'lucide-vue-next'
import ProbeHistoryStrip from './ProbeHistoryStrip.vue'
import QualityHistoryStrip from './QualityHistoryStrip.vue'
import { connectionHealthStateBadgeClass } from '../../composables/useConnectionHealth'
import type { QualityManualMethod } from '../../types/quality'
import type { AdminGroupAccount } from '../../types/connectionHealth'
import { channelAutomationEnabled, latestChannelProbe } from '../../utils/connectionHealthChannels'

const props = defineProps<{ account: AdminGroupAccount; historyUnavailable?: boolean; showQuality?: boolean; qualityEnabled?: boolean; qualityUnavailable?: boolean; qualityBusy?: boolean; qualityProbeMethod?: QualityManualMethod; qualityError?: string; suspensionBusy?: boolean; suspensionError?: string; priorityBusy?: boolean; priorityError?: string }>()
const emit = defineEmits<{ probe: [account: AdminGroupAccount]; 'view-events': [account: AdminGroupAccount]; 'view-quality': [account: AdminGroupAccount]; 'toggle-quality': [account: AdminGroupAccount]; 'probe-quality': [account: AdminGroupAccount, method: QualityManualMethod]; 'toggle-suspension': [account: AdminGroupAccount]; 'toggle-priority': [account: AdminGroupAccount] }>()
const { t } = useI18n()
const prefix = 'admin.connectionHealth'
const qualityMethods: QualityManualMethod[] = ['questions', 'manxue_candy', 'manxue_pelican']
const selectedForAutomation = computed(() => channelAutomationEnabled(props.account))
const hasManualProbes = computed(() => props.account.recentProbes?.some(sample => sample.manual))
const hasManualQuality = computed(() => props.account.qualityHistory?.some(sample => sample.manual))
const automaticQualityEnabled = computed(() => selectedForAutomation.value && props.account.qualityEnabled !== false)
const canProbeQuality = computed(() => props.account.probeAvailable && !props.account.qualityPausedByHealth && !props.qualityUnavailable)
const latest = computed(() => latestChannelProbe(props.account))
const latencyPrefix = `${prefix}.latencyPriority`
const exhaustedBudgets = computed(() => props.account.probeBudgets?.filter(budget => budget.exhausted) ?? [])
const budgetResetTime = (value: string) => new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
const priorityDecision = computed(() => props.account.latencyPriority)
const sampleWeight = (index: number): string => {
  const d = priorityDecision.value
  if (!d) return '0'
  const sum = d.weights.slice(0, d.sampleCount).reduce((a, b) => a + b, 0)
  return sum > 0 ? ((d.weights[index] ?? 0) / sum * 100).toFixed(1) : '0'
}
const state = computed(() => {
  if (props.account.qualitySuspended) return 'qualitySuspended'
  if (props.historyUnavailable) return 'loadError'
  if (!props.account.probeAvailable) return 'unavailable'
  if (!latest.value) return 'pending'
  return latest.value.result === 'ok' ? 'healthy' : 'unhealthy'
})
</script>

<template>
  <section class="min-w-0 space-y-3 rounded-lg border border-border/60 bg-card p-4" :aria-label="account.name || account.id">
    <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
      <div class="min-w-[8rem] max-w-full flex-1">
        <h3 class="truncate text-sm font-medium text-foreground" :title="account.name || account.id">{{ account.name || account.id }}</h3>
      </div>
      <div class="flex max-w-full flex-wrap items-center gap-x-2 gap-y-1">
        <button v-if="selectedForAutomation" type="button" role="switch" :aria-checked="account.priorityEnabled !== false" :aria-label="t(`${prefix}.channelPriority.toggle`, { name: account.name || account.id })" :title="t(`${prefix}.channelPriority.hint`)" :disabled="priorityBusy" class="flex items-center gap-1.5 rounded px-1 py-2 text-xs text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50" @click.stop="emit('toggle-priority', account)">
          <span>{{ t(`${prefix}.channelPriority.label`) }}</span>
          <Loader2 v-if="priorityBusy" class="h-4 w-7 animate-spin" />
          <span v-else class="relative h-4 w-7 rounded-full transition-colors" :class="account.priorityEnabled !== false ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="account.priorityEnabled !== false ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
        </button>
        <button v-if="selectedForAutomation && account.suspensionSupported" type="button" role="switch" :aria-checked="account.suspensionEnabled !== false" :aria-label="t(`${prefix}.channelSuspension.toggle`, { name: account.name || account.id })" :title="t(`${prefix}.channelSuspension.hint`)" :disabled="suspensionBusy" class="flex items-center gap-1.5 rounded px-1 py-2 text-xs text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50" @click.stop="emit('toggle-suspension', account)">
          <span>{{ t(`${prefix}.channelSuspension.label`) }}</span>
          <Loader2 v-if="suspensionBusy" class="h-4 w-7 animate-spin" />
          <span v-else class="relative h-4 w-7 rounded-full transition-colors" :class="account.suspensionEnabled !== false ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="account.suspensionEnabled !== false ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
        </button>
        <button type="button" role="switch" :aria-checked="automaticQualityEnabled" :aria-label="t(`${prefix}.quality.toggleChannel`, { name: account.name || account.id })" :title="t(`${prefix}.quality.${selectedForAutomation ? 'channelSwitchHint' : 'inactiveAutomationHint'}`)" :disabled="!selectedForAutomation || qualityBusy || qualityUnavailable" class="flex h-6 shrink-0 items-center justify-center gap-1.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50" @click.stop="emit('toggle-quality', account)">
          <span class="text-xs text-muted-foreground">{{ t(`${prefix}.quality.automaticLabel`) }}</span>
          <Loader2 v-if="qualityBusy" class="h-4 w-4 animate-spin text-muted-foreground" />
          <span v-else class="relative h-4 w-7 rounded-full transition-colors" :class="automaticQualityEnabled ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="automaticQualityEnabled ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
        </button>
        <button v-for="method in qualityMethods" :key="method" type="button" class="inline-flex h-6 shrink-0 items-center justify-center gap-1 rounded border border-border/60 px-1.5 text-xs text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-40" :disabled="qualityBusy || Boolean(qualityProbeMethod) || !canProbeQuality" :aria-busy="qualityProbeMethod === method" :aria-label="t(`${prefix}.quality.manualLabels.${method}`)" :title="t(`${prefix}.quality.${qualityProbeMethod === method ? 'probingNow' : account.qualityPausedByHealth ? 'healthPausedHint' : canProbeQuality ? `manualHints.${method}` : 'probeUnavailable'}`)" @click.stop="emit('probe-quality', account, method)">
          <Loader2 v-if="qualityProbeMethod === method" class="h-3 w-3 animate-spin" aria-hidden="true" />
          <Play v-else class="h-3 w-3" aria-hidden="true" />
          {{ t(`${prefix}.quality.manualLabels.${method}`) }}
        </button>
        <button type="button" class="inline-flex h-6 shrink-0 items-center gap-1 rounded px-1.5 text-xs text-primary hover:bg-primary/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :aria-label="t(`${prefix}.quality.detail.open`, { name: account.name || account.id })" @click.stop="emit('view-quality', account)"><Eye class="h-3.5 w-3.5" />{{ t(`${prefix}.quality.detail.channelButton`) }}</button>
        <span class="rounded-full px-2.5 py-1 text-xs" :class="state === 'healthy' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : state === 'unhealthy' || state === 'qualitySuspended' ? 'bg-red-500/10 text-red-600 dark:text-red-400' : 'bg-surface text-muted-foreground'">{{ t(`${prefix}.cards.status.${state}`) }}</span>
        <button type="button" class="inline-flex h-6 items-center gap-1 rounded border border-border/60 px-1.5 text-xs text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" :disabled="!account.probeAvailable" :aria-label="t(`${prefix}.actions.probe`)" :title="t(`${prefix}.actions.probe`)" @click.stop="emit('probe', account)"><Zap class="h-3 w-3" />{{ t(`${prefix}.actions.probeShort`) }}</button>
        <button type="button" class="rounded-lg border border-border/70 p-2 text-muted-foreground hover:text-primary" :aria-label="t(`${prefix}.actions.viewEvents`)" :title="t(`${prefix}.actions.viewEvents`)" @click="emit('view-events', account)"><Eye class="h-3.5 w-3.5" /></button>
      </div>
    </div>
    <p v-if="qualityError" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ qualityError }}</p>
    <p v-if="priorityError" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ priorityError }}</p>
    <p v-if="account.priorityRestorePending" role="status" class="text-xs text-amber-600 dark:text-amber-400">{{ t(`${prefix}.channelPriority.restoring`) }}</p>
    <p v-if="suspensionError" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ suspensionError }}</p>
    <p v-for="budget in exhaustedBudgets" :key="budget.policyId" role="status" class="break-words text-xs text-amber-600 dark:text-amber-400" :title="budget.policyName">
      {{ t(`${prefix}.probeBudget.exhausted`, { models: budget.models.join('、'), used: budget.used, limit: budget.limit, time: budgetResetTime(budget.resetsAt) }) }}
    </p>
    <p v-if="account.probeBudgetError" role="status" class="text-xs text-amber-600 dark:text-amber-400">{{ t(`${prefix}.probeBudget.unavailable`) }}</p>
    <ProbeHistoryStrip v-if="selectedForAutomation || hasManualProbes" :samples="account.recentProbes" :unavailable="historyUnavailable">
      <template #before-stats>
        <span class="text-muted-foreground">{{ t(`${prefix}.groupDetail.columns.priority`) }} {{ account.priority ?? '—' }}</span>
      </template>
      <template #after-stats>
        <span v-for="model in account.modelHealth" :key="model.modelName" class="whitespace-nowrap rounded-md px-2 py-1 text-xs" :class="connectionHealthStateBadgeClass(model.state)">{{ model.modelName }} · {{ t(`${prefix}.stateLabels.${model.state}`) }}</span>
        <span v-for="model in account.unprobedModels" :key="`pending-${model.modelName}`" class="whitespace-nowrap rounded-md bg-surface px-2 py-1 text-xs text-muted-foreground">{{ model.modelName }} · {{ t(`${prefix}.notProbed`) }}</span>
      </template>
    </ProbeHistoryStrip>
    <details v-if="selectedForAutomation && priorityDecision" class="rounded-md bg-surface/40 px-2 py-1.5 text-xs">
      <summary class="flex cursor-pointer flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground">
        <span>{{ t(`${latencyPrefix}.${priorityDecision.probeMode === 'first_token' ? 'averageFirstToken' : 'average'}`) }} <strong class="font-semibold text-foreground">{{ priorityDecision.averageMs != null ? `${(priorityDecision.averageMs / 1000).toFixed(2)}s` : '—' }}</strong></span>
        <span>{{ t(`${latencyPrefix}.samples`, { used: priorityDecision.sampleCount, total: priorityDecision.requiredSamples }) }}</span>
        <span v-if="account.priorityEnabled !== false">{{ t(`${latencyPrefix}.decision`, { priority: priorityDecision.priority }) }}</span>
        <span v-else>{{ t(`${prefix}.channelPriority.off`) }}</span>
        <span class="ml-auto text-primary">{{ t(`${latencyPrefix}.details`) }}</span>
      </summary>
      <div class="mt-2 space-y-1.5 break-words border-t border-border/40 pt-2 text-muted-foreground">
        <p>{{ t(`${latencyPrefix}.policyLine`, { name: priorityDecision.policyName }) }} · {{ t(`admin.connectionHealth.policyDrawer.probeModes.${priorityDecision.probeMode || 'real_model'}`) }}</p>
        <p>{{ t(`${latencyPrefix}.modelLine`, { model: priorityDecision.modelName, seconds: priorityDecision.maxAgeSeconds }) }}</p>
        <p>{{ t(`${latencyPrefix}.reasons.${priorityDecision.reason}`, { n: (priorityDecision.bandIndex ?? 0) + 1 }) }}</p>
        <p v-if="priorityDecision.sharedPolicyCount > 1">{{ t(`${latencyPrefix}.shared`, { count: priorityDecision.sharedPolicyCount }) }}</p>
        <p v-if="account.priorityConflict" class="text-amber-600 dark:text-amber-400">{{ t(`${latencyPrefix}.conflict`) }}</p>
        <p v-for="(sample, i) in priorityDecision.samples" :key="sample.id">{{ t(`${latencyPrefix}.sampleLine`, { time: new Date(sample.createdAt).toLocaleTimeString(), seconds: (sample.latencyMs / 1000).toFixed(2), weight: sampleWeight(i) }) }}</p>
      </div>
    </details>
    <p v-if="selectedForAutomation && account.latencyPriorityError" class="text-xs text-amber-600 dark:text-amber-400">{{ t(`${latencyPrefix}.unavailable`) }}</p>
    <template v-if="(selectedForAutomation && showQuality) || hasManualQuality || account.qualitySuspended">
      <QualityHistoryStrip :samples="account.qualityHistory" :state="account.qualityState" :paused-by-health="account.qualityPausedByHealth" :suspended-by-quality="account.qualitySuspended" :enabled="qualityEnabled && account.qualityEnabled !== false" :selected="account.qualitySelected" :unavailable="qualityUnavailable" />
    </template>
  </section>
</template>
