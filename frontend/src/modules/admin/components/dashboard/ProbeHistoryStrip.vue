<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatConnectionHealthTime } from '../../composables/useConnectionHealth'
import type { GroupProbeSample } from '../../types/connectionHealth'

const props = withDefaults(defineProps<{ samples?: GroupProbeSample[]; unavailable?: boolean; limit?: number; compact?: boolean }>(), { limit: 100 })
const { t } = useI18n()
const prefix = 'admin.connectionHealth.cards'
const samples = computed(() => [...(props.samples ?? [])]
  .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt) || b.id.localeCompare(a.id)).slice(0, props.limit))
const history = computed(() => [...Array.from({ length: props.limit - samples.value.length }, () => null), ...samples.value.slice().reverse()])
const successes = computed(() => samples.value.filter(sample => sample.result === 'ok'))
const errors = computed(() => samples.value.length - successes.value.length)
const slow = computed(() => successes.value.filter(sample => sample.latencyMs != null && sample.latencyMs >= 5000).length)
const successRate = computed(() => samples.value.length ? `${Math.round(successes.value.length / samples.value.length * 100)}%` : '—')
const averageLatency = computed(() => {
  const values = successes.value.flatMap(sample => sample.latencyMs == null ? [] : [sample.latencyMs])
  return values.length ? `${(values.reduce((sum, value) => sum + value, 0) / values.length / 1000).toFixed(2)}s` : '—'
})
const sampleTone = (sample: GroupProbeSample | null) => !sample ? 'bg-slate-200/70 dark:bg-slate-700/60'
  : sample.result !== 'ok' ? 'bg-red-500 dark:bg-red-400'
    : sample.latencyMs != null && sample.latencyMs >= 5000 ? 'bg-amber-400' : 'bg-emerald-500 dark:bg-emerald-400'
const sampleTitle = (sample: GroupProbeSample | null) => !sample ? t(`${prefix}.noRecord`)
  : `${sample.modelName} · ${t(`admin.connectionHealth.errorKeys.${sample.result}`)}\n${sample.latencyMs == null ? '—' : `${(sample.latencyMs / 1000).toFixed(2)}s`} · ${formatConnectionHealthTime(sample.createdAt)}`
const lastProbe = computed(() => {
  const latest = samples.value[0]?.createdAt
  if (!latest) return ''
  const elapsedMinutes = Math.max(0, Math.floor((Date.now() - Date.parse(latest)) / 60_000))
  if (elapsedMinutes < 1) return t(`${prefix}.justNow`)
  if (elapsedMinutes < 60) return t(`${prefix}.minutesAgo`, { count: elapsedMinutes })
  return formatConnectionHealthTime(latest)
})
</script>

<template>
  <div :class="compact ? 'space-y-1.5' : 'space-y-3'">
    <div v-if="compact" class="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
      <span>{{ t(`${prefix}.recent`, { count: samples.length }) }}</span>
      <span v-if="samples[0]" :title="sampleTitle(samples[0])">{{ samples[0].latencyMs == null ? '—' : `${(samples[0].latencyMs / 1000).toFixed(2)}s` }} · {{ lastProbe }}</span>
    </div>
    <div v-else class="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 text-xs sm:text-sm">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 tabular-nums">
        <span class="text-muted-foreground">{{ t(`${prefix}.recent`, { count: samples.length }) }}</span>
        <slot name="before-stats" />
        <template v-if="samples.length">
          <span :class="slow ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'">{{ t(`${prefix}.slow`, { count: slow }) }}</span>
          <span :class="errors ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'">{{ t(`${prefix}.errors`, { count: errors }) }}</span>
          <span class="text-emerald-700 dark:text-emerald-400">{{ t(`${prefix}.successRate`, { value: successRate }) }}</span>
        </template>
        <slot name="after-stats" />
        <span v-if="lastProbe" class="text-xs text-muted-foreground">{{ lastProbe }}</span>
      </div>
      <span class="whitespace-nowrap text-muted-foreground" :title="t(`${prefix}.latencyHint`)">{{ t(`${prefix}.averageLatency`) }} <strong class="ml-1 font-semibold tabular-nums text-foreground">{{ averageLatency }}</strong></span>
    </div>
    <div class="flex gap-px overflow-hidden rounded sm:gap-[2px]" :class="compact ? 'h-3' : 'h-5'" role="img" :aria-label="unavailable ? t(`${prefix}.historyUnavailable`) : t(`${prefix}.historyLabel`, { count: samples.length, errors, rate: successRate })">
      <span v-for="(sample, index) in history" :key="sample?.id ?? `empty-${index}`" class="min-w-0 flex-1 rounded-[1px] transition-opacity hover:opacity-60" :class="sampleTone(sample)" :title="sampleTitle(sample)" aria-hidden="true" />
    </div>
    <p v-if="unavailable || (!compact && !samples.length)" class="text-xs text-muted-foreground">{{ t(`${prefix}.${unavailable ? 'historyUnavailable' : 'noHistory'}`) }}</p>
  </div>
</template>
