<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, Loader2, Play, Settings2, Zap } from 'lucide-vue-next'
import ProbeHistoryStrip from './ProbeHistoryStrip.vue'
import ChannelActionDropdown from './ChannelActionDropdown.vue'
import QualityHistoryStrip from './QualityHistoryStrip.vue'
import { connectionHealthStateBadgeClass } from '../../composables/useConnectionHealth'
import type { QualityManualMethod } from '../../types/quality'
import type { AdminGroupAccount } from '../../types/connectionHealth'
import { channelAutomationEnabled, latestChannelProbe } from '../../utils/connectionHealthChannels'

const props = defineProps<{ account: AdminGroupAccount; historyUnavailable?: boolean; showQuality?: boolean; qualityEnabled?: boolean; qualityUnavailable?: boolean; qualityBusy?: boolean; qualityProbeMethod?: QualityManualMethod; qualityError?: string; suspensionBusy?: boolean; suspensionError?: string; priorityBusy?: boolean; priorityError?: string; autoProbeBusy?: boolean; autoProbeError?: string; qualitySuspensionBusy?: boolean; qualitySuspensionError?: string }>()
const emit = defineEmits<{ probe: [account: AdminGroupAccount]; 'view-events': [account: AdminGroupAccount]; 'view-quality': [account: AdminGroupAccount]; 'toggle-quality': [account: AdminGroupAccount]; 'probe-quality': [account: AdminGroupAccount, method: QualityManualMethod]; 'toggle-suspension': [account: AdminGroupAccount]; 'toggle-priority': [account: AdminGroupAccount]; 'toggle-auto-probe': [account: AdminGroupAccount]; 'toggle-quality-suspension': [account: AdminGroupAccount] }>()
const { t } = useI18n()
const prefix = 'admin.connectionHealth'
const qualityMethods: QualityManualMethod[] = ['questions', 'manxue_candy', 'manxue_pelican']
const selectedForAutomation = computed(() => channelAutomationEnabled(props.account))
const hasManualProbes = computed(() => props.account.recentProbes?.some(sample => sample.manual))
const hasManualQuality = computed(() => props.account.qualityHistory?.some(sample => sample.manual))
const automaticQualityEnabled = computed(() => selectedForAutomation.value && props.account.qualityEnabled !== false)
const automaticProbeEnabled = computed(() => selectedForAutomation.value && props.account.hasEnabledProbePolicy !== false && props.account.autoProbeEnabled !== false)
const automationControls = computed(() => [
  { id: 'probe', label: t(`${prefix}.channelAutoProbe.label`), aria: t(`${prefix}.channelAutoProbe.toggle`, { name: props.account.name || props.account.id }), hint: t(`${prefix}.channelAutoProbe.hint`), checked: automaticProbeEnabled.value, busy: props.autoProbeBusy, disabled: !selectedForAutomation.value || props.account.hasEnabledProbePolicy === false || props.autoProbeBusy, toggle: () => emit('toggle-auto-probe', props.account) },
  { id: 'priority', label: t(`${prefix}.channelPriority.label`), aria: t(`${prefix}.channelPriority.toggle`, { name: props.account.name || props.account.id }), hint: t(`${prefix}.channelPriority.hint`), checked: selectedForAutomation.value && props.account.priorityEnabled !== false, busy: props.priorityBusy, disabled: !selectedForAutomation.value || props.priorityBusy, toggle: () => emit('toggle-priority', props.account) },
  { id: 'suspension', label: t(`${prefix}.channelSuspension.label`), aria: t(`${prefix}.channelSuspension.toggle`, { name: props.account.name || props.account.id }), hint: t(`${prefix}.channelSuspension.hint`), checked: selectedForAutomation.value && Boolean(props.account.suspensionSupported) && props.account.suspensionEnabled !== false, busy: props.suspensionBusy, disabled: !selectedForAutomation.value || !props.account.suspensionSupported || props.suspensionBusy, toggle: () => emit('toggle-suspension', props.account) },
  { id: 'quality-suspension', label: t(`${prefix}.channelQualitySuspension.label`), aria: t(`${prefix}.channelQualitySuspension.toggle`, { name: props.account.name || props.account.id }), hint: t(`${prefix}.channelQualitySuspension.hint`), checked: Boolean(props.account.qualitySuspensionEnabled), busy: props.qualitySuspensionBusy, disabled: props.qualitySuspensionBusy || (!props.account.qualitySuspensionEnabled && (!selectedForAutomation.value || !props.account.suspensionSupported)), toggle: () => emit('toggle-quality-suspension', props.account) },
  { id: 'quality', label: t(`${prefix}.quality.automaticLabel`), aria: t(`${prefix}.quality.toggleChannel`, { name: props.account.name || props.account.id }), hint: t(`${prefix}.quality.${selectedForAutomation.value ? 'channelSwitchHint' : 'inactiveAutomationHint'}`), checked: automaticQualityEnabled.value, busy: props.qualityBusy, disabled: !selectedForAutomation.value || props.qualityBusy || props.qualityUnavailable, toggle: () => emit('toggle-quality', props.account) },
])
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
        <ChannelActionDropdown :label="t(`${prefix}.channelMenus.automationFor`, { name: account.name || account.id })">
          <template #trigger><Settings2 class="h-3.5 w-3.5" />{{ t(`${prefix}.channelMenus.automation`) }}<span v-if="selectedForAutomation && account.autoProbeEnabled === false && account.hasEnabledProbePolicy !== false" class="text-amber-600 dark:text-amber-400">· {{ t(`${prefix}.channelAutoProbe.off`) }}</span></template>
          <button v-for="control in automationControls" :key="control.id" type="button" role="switch" :aria-checked="control.checked" :aria-label="control.aria" :title="control.hint" :disabled="control.disabled" class="flex w-full items-center justify-between gap-3 rounded-md px-3 py-2.5 text-xs hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-40" @click.stop="control.toggle()">
            <span>{{ control.label }}</span>
            <Loader2 v-if="control.busy" class="h-4 w-7 animate-spin" />
            <span v-else class="relative h-4 w-7 shrink-0 rounded-full transition-colors" :class="control.checked ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="control.checked ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
          </button>
          <p v-if="!selectedForAutomation" class="border-t border-border/60 px-3 py-2 text-[11px] leading-5 text-muted-foreground">{{ t(`${prefix}.quality.inactiveAutomationHint`) }}</p>
        </ChannelActionDropdown>
        <ChannelActionDropdown :label="t(`${prefix}.channelMenus.qualityFor`, { name: account.name || account.id })">
          <template #trigger><Loader2 v-if="qualityProbeMethod" class="h-3.5 w-3.5 animate-spin" /><Play v-else class="h-3.5 w-3.5" />{{ t(`${prefix}.channelMenus.quality`) }}</template>
          <template #default="{ close }">
            <button v-for="method in qualityMethods" :key="method" type="button" class="flex w-full items-center gap-2 rounded-md px-3 py-2.5 text-xs text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-40" :disabled="qualityBusy || Boolean(qualityProbeMethod) || !canProbeQuality" :aria-busy="qualityProbeMethod === method" :aria-label="t(`${prefix}.quality.manualLabels.${method}`)" :title="t(`${prefix}.quality.${qualityProbeMethod === method ? 'probingNow' : account.qualityPausedByHealth ? 'healthPausedHint' : canProbeQuality ? `manualHints.${method}` : 'probeUnavailable'}`)" @click.stop="close(); emit('probe-quality', account, method)">
              <Loader2 v-if="qualityProbeMethod === method" class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
              <Play v-else class="h-3.5 w-3.5" aria-hidden="true" />
              {{ t(`${prefix}.quality.manualLabels.${method}`) }}
            </button>
          </template>
        </ChannelActionDropdown>
        <button type="button" class="inline-flex h-6 shrink-0 items-center gap-1 rounded px-1.5 text-xs text-primary hover:bg-primary/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :aria-label="t(`${prefix}.quality.detail.open`, { name: account.name || account.id })" @click.stop="emit('view-quality', account)"><Eye class="h-3.5 w-3.5" />{{ t(`${prefix}.quality.detail.channelButton`) }}</button>
        <span class="rounded-full px-2.5 py-1 text-xs" :class="state === 'healthy' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : state === 'unhealthy' || state === 'qualitySuspended' ? 'bg-red-500/10 text-red-600 dark:text-red-400' : 'bg-surface text-muted-foreground'">{{ t(`${prefix}.cards.status.${state}`) }}</span>
        <button type="button" class="inline-flex h-6 items-center gap-1 rounded border border-border/60 px-1.5 text-xs text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" :disabled="!account.probeAvailable" :aria-label="t(`${prefix}.actions.probe`)" :title="t(`${prefix}.actions.probe`)" @click.stop="emit('probe', account)"><Zap class="h-3 w-3" />{{ t(`${prefix}.actions.probeShort`) }}</button>
        <button type="button" class="rounded-lg border border-border/70 p-2 text-muted-foreground hover:text-primary" :aria-label="t(`${prefix}.actions.viewEvents`)" :title="t(`${prefix}.actions.viewEvents`)" @click="emit('view-events', account)"><Eye class="h-3.5 w-3.5" /></button>
      </div>
    </div>
    <p v-if="qualitySuspensionError" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ qualitySuspensionError }}</p>
    <p v-if="account.qualitySuspensionRestorePending" role="status" class="text-xs text-amber-600 dark:text-amber-400">{{ t(`${prefix}.channelQualitySuspension.restoring`) }}</p>
    <p v-if="autoProbeError" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ autoProbeError }}</p>
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
