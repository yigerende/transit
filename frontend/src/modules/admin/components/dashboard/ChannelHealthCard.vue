<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Eye, Zap } from 'lucide-vue-next'
import ProbeHistoryStrip from './ProbeHistoryStrip.vue'
import QualityHistoryStrip from './QualityHistoryStrip.vue'
import { connectionHealthStateBadgeClass } from '../../composables/useConnectionHealth'
import type { AdminGroupAccount } from '../../types/connectionHealth'

const props = defineProps<{ account: AdminGroupAccount; historyUnavailable?: boolean; showQuality?: boolean; qualityEnabled?: boolean; qualityUnavailable?: boolean }>()
const emit = defineEmits<{ probe: [account: AdminGroupAccount]; 'view-events': [account: AdminGroupAccount] }>()
const { t } = useI18n()
const prefix = 'admin.connectionHealth'
// This flag follows saved channel selection, independently of policy on/off.
// Fall back to assignment metadata when connected to an older backend.
const selectedForAutomation = computed(() => props.account.qualitySelected
  ?? (!props.account.excludedFromGroupPolicy && Boolean(props.account.hasAssignedPolicy
    ?? props.account.assignedPolicyIds?.length
    ?? props.account.assignedPolicies?.length)))
const latest = computed(() => props.account.recentProbes?.[0])
const state = computed(() => {
  if (props.historyUnavailable) return 'loadError'
  if (!props.account.probeAvailable) return 'unavailable'
  if (!latest.value) return 'pending'
  return latest.value.result === 'ok' ? 'healthy' : 'unhealthy'
})
</script>

<template>
  <section class="min-w-0 space-y-3 rounded-lg border border-border/60 bg-card p-4" :aria-label="account.name || account.id">
    <div class="flex items-center justify-between gap-2" :class="selectedForAutomation ? 'flex-wrap' : ''">
      <div class="min-w-0" :class="selectedForAutomation ? '' : 'flex-1'">
        <h3 class="text-sm font-medium text-foreground" :class="selectedForAutomation ? 'break-words' : 'truncate'" :title="account.name || account.id">{{ account.name || account.id }}</h3>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <span class="rounded-full px-2.5 py-1 text-xs" :class="state === 'healthy' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : state === 'unhealthy' ? 'bg-red-500/10 text-red-600 dark:text-red-400' : 'bg-surface text-muted-foreground'">{{ t(`${prefix}.cards.status.${state}`) }}</span>
        <button type="button" class="rounded-lg border border-border/70 p-2 text-muted-foreground hover:text-primary disabled:opacity-40" :disabled="!account.probeAvailable" :aria-label="t(`${prefix}.actions.probe`)" :title="t(`${prefix}.actions.probe`)" @click="emit('probe', account)"><Zap class="h-3.5 w-3.5" /></button>
        <button type="button" class="rounded-lg border border-border/70 p-2 text-muted-foreground hover:text-primary" :aria-label="t(`${prefix}.actions.viewEvents`)" :title="t(`${prefix}.actions.viewEvents`)" @click="emit('view-events', account)"><Eye class="h-3.5 w-3.5" /></button>
      </div>
    </div>
    <ProbeHistoryStrip v-if="selectedForAutomation" :samples="account.recentProbes" :unavailable="historyUnavailable">
      <template #before-stats>
        <span class="text-muted-foreground">{{ t(`${prefix}.groupDetail.columns.priority`) }} {{ account.priority ?? '—' }}</span>
      </template>
    </ProbeHistoryStrip>
    <QualityHistoryStrip v-if="selectedForAutomation && showQuality" :samples="account.qualityHistory" :state="account.qualityState" :enabled="qualityEnabled" :selected="account.qualitySelected" :unavailable="qualityUnavailable" />
    <div v-if="selectedForAutomation && (account.modelHealth.length || account.unprobedModels?.length)" class="flex flex-wrap gap-2">
      <span v-for="model in account.modelHealth" :key="model.modelName" class="rounded-md px-2 py-1 text-xs" :class="connectionHealthStateBadgeClass(model.state)">{{ model.modelName }} · {{ t(`${prefix}.stateLabels.${model.state}`) }}</span>
      <span v-for="model in account.unprobedModels" :key="`pending-${model.modelName}`" class="rounded-md bg-surface px-2 py-1 text-xs text-muted-foreground">{{ model.modelName }} · {{ t(`${prefix}.notProbed`) }}</span>
    </div>
  </section>
</template>
