<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { connectionHealthMessageKey, formatConnectionHealthTime } from '../../composables/useConnectionHealth'
import type { QualitySample, QualityState } from '../../types/quality'
const props = defineProps<{ samples?: QualitySample[]; state?: QualityState; enabled?: boolean; selected?: boolean; unavailable?: boolean }>()
const { t, te } = useI18n()
const p = 'admin.connectionHealth.quality'
const samples = computed(() => [...(props.samples || [])].sort((a,b) => Date.parse(b.createdAt)-Date.parse(a.createdAt) || b.id.localeCompare(a.id)).slice(0,100))
const history = computed(() => [...Array.from({ length:100-samples.value.length }, () => null), ...samples.value.slice().reverse()])
const passed = computed(() => samples.value.filter(s => s.result === 'passed').length)
const failed = computed(() => samples.value.filter(s => s.result === 'failed').length)
const errors = computed(() => samples.value.filter(s => s.result === 'error').length)
const rate = computed(() => passed.value + failed.value ? `${Math.round(passed.value/(passed.value+failed.value)*100)}%` : '—')
const status = computed(() => props.unavailable ? 'unavailable' : !props.selected ? 'notSelected' : !props.enabled ? 'off' : props.state?.status || 'pending')
const color = (sample: QualitySample | null) => !sample ? 'bg-slate-200/70 dark:bg-slate-700/60' : sample.result === 'passed' ? 'bg-emerald-500 dark:bg-emerald-400' : sample.result === 'failed' ? 'bg-red-500 dark:bg-red-400' : 'bg-amber-400'
function title(sample: QualitySample | null) {
  if (!sample) return t(`${p}.noRecord`)
  if (sample.detectionMethod === 'manxue') {
    const result = sample.errorKey ? t(connectionHealthMessageKey(sample.errorKey, te)) : t(`${p}.${sample.result}`)
    return `${t(`${p}.methodManxue`)} · ${t(`${p}.benchmarks.${sample.benchmark || 'candy'}`)} · ${sample.model}\n${result} · ${(sample.durationMs/1000).toFixed(2)}s\n${sample.report || sample.answer || ''}\n${formatConnectionHealthTime(sample.createdAt)}`
  }
  const detail = sample.errorKey ? t(connectionHealthMessageKey(sample.errorKey,te)) : `${t(`${p}.actualAnswer`)}: ${sample.answer}\n${t(`${p}.expectedAnswer`)}: ${sample.expectedAnswer}\n${t(`${p}.contentResult`)}: ${t(`${p}.${sample.contentPassed?'passed':'failed'}`)} · ${t(`${p}.timeResult`)}: ${t(`${p}.${sample.timePassed?'passed':'failed'}`)} (${sample.durationMs} / <${sample.maxDurationMs} ms)`
  return `${sample.questionName} · ${sample.model}\n${t(`${p}.${sample.result}`)} · ${(sample.durationMs/1000).toFixed(2)}s\n${detail}\n${formatConnectionHealthTime(sample.createdAt)}`
}
</script>
<template>
  <div class="space-y-3 border-t border-border/40 pt-3">
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 tabular-nums">
        <span class="font-medium text-muted-foreground">{{ t(`${p}.stripTitle`) }}</span>
        <slot name="controls" />
        <span :class="selected && enabled && state?.degraded ? 'text-red-600 dark:text-red-400' : status === 'normal' ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'">{{ t(`${p}.statuses.${status}`) }}</span>
        <template v-if="samples.length"><span class="text-muted-foreground">{{ t(`${p}.recent`, { count:samples.length }) }}</span><span :class="failed ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'">{{ t(`${p}.failedCount`, { count:failed }) }}</span><span :class="errors ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'">{{ t(`${p}.errorCount`, { count:errors }) }}</span><span class="text-emerald-700 dark:text-emerald-400" :title="t(`${p}.rateHint`)">{{ t(`${p}.passRate`, { rate }) }}</span></template>
      </div>
      <span v-if="state" class="text-muted-foreground" :title="title(state.latest)">{{ state.latest.model }} · {{ (state.latest.durationMs/1000).toFixed(2) }}s</span>
    </div>
    <div class="flex h-5 gap-px overflow-hidden rounded sm:gap-[2px]" role="img" :aria-label="t(`${p}.historyLabel`, { count:samples.length, failed, errors, rate })">
      <span v-for="(sample,index) in history" :key="sample?.id ?? `empty-${index}`" class="min-w-0 flex-1 rounded-[1px] transition-opacity hover:opacity-60" :class="color(sample)" :title="title(sample)" aria-hidden="true" />
    </div>
    <p v-if="unavailable || !selected || !samples.length" class="text-xs text-muted-foreground">{{ t(`${p}.${unavailable ? 'historyUnavailable' : !selected ? 'selectionHint' : enabled ? 'waiting' : 'enableHint'}`) }}</p>
    <p v-else-if="state?.latest.errorKey" class="text-xs text-amber-600 dark:text-amber-400">{{ t(connectionHealthMessageKey(state.latest.errorKey,te)) }}<span v-if="state.degraded"> · {{ t(`${p}.previousDegraded`) }}</span></p>
  </div>
</template>
