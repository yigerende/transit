<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, useId, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { connectionHealthMessageKey, formatConnectionHealthTime } from '../../composables/useConnectionHealth'
import type { QualitySample, QualityState } from '../../types/quality'
const props = defineProps<{ samples?: QualitySample[]; state?: QualityState; enabled?: boolean; pausedByHealth?: boolean; selected?: boolean; unavailable?: boolean }>()
const { t, te } = useI18n()
const p = 'admin.connectionHealth.quality'
const samples = computed(() => [...(props.samples || [])].sort((a,b) => Date.parse(b.createdAt)-Date.parse(a.createdAt) || b.id.localeCompare(a.id)).slice(0,100))
const history = computed(() => [...Array.from({ length:100-samples.value.length }, () => null), ...samples.value.slice().reverse()])
const passed = computed(() => samples.value.filter(s => s.result === 'passed').length)
const failed = computed(() => samples.value.filter(s => s.result === 'failed').length)
const errors = computed(() => samples.value.filter(s => s.result === 'error').length)
const rate = computed(() => passed.value + failed.value ? `${Math.round(passed.value/(passed.value+failed.value)*100)}%` : '—')
const manualOnly = computed(() => (!props.selected || !props.enabled) && samples.value.some(sample => sample.manual))
const status = computed(() => props.unavailable ? 'unavailable' : props.pausedByHealth ? 'healthPaused' : manualOnly.value ? 'manualOnly' : !props.selected ? 'notSelected' : !props.enabled ? 'off' : props.state?.status || 'pending')
const color = (sample: QualitySample | null) => !sample ? 'bg-slate-200/70 dark:bg-slate-700/60' : sample.result === 'passed' ? 'bg-emerald-500 dark:bg-emerald-400' : sample.result === 'failed' ? 'bg-red-500 dark:bg-red-400' : 'bg-amber-400'
function title(sample: QualitySample | null) {
  if (!sample) return t(`${p}.noRecord`)
  if (sample.detectionMethod === 'manxue') {
    const result = sample.errorKey ? t(connectionHealthMessageKey(sample.errorKey, te)) : t(`${p}.${sample.result}`)
    return `${formatConnectionHealthTime(sample.createdAt)}\n${t(`${p}.methodManxue`)} · ${t(`${p}.benchmarks.${sample.benchmark || 'candy'}`)} · ${sample.model}\n${result} · ${(sample.durationMs/1000).toFixed(2)}s\n${sample.report || sample.answer || ''}`
  }
  const detail = sample.errorKey ? t(connectionHealthMessageKey(sample.errorKey,te)) : `${t(`${p}.actualAnswer`)}: ${sample.answer}\n${t(`${p}.expectedAnswer`)}: ${sample.expectedAnswer}\n${t(`${p}.contentResult`)}: ${t(`${p}.${sample.contentPassed?'passed':'failed'}`)} · ${t(`${p}.timeResult`)}: ${t(`${p}.${sample.timePassed?'passed':'failed'}`)} (${sample.durationMs} / <${sample.maxDurationMs} ms)`
  return `${formatConnectionHealthTime(sample.createdAt)}\n${sample.questionName} · ${sample.model}\n${t(`${p}.${sample.result}`)} · ${(sample.durationMs/1000).toFixed(2)}s\n${detail}`
}

const strip = ref<HTMLElement | null>(null)
const tooltip = ref<HTMLElement | null>(null)
const tooltipId = useId()
const activeIndex = ref<number | null>(null)
// Keep the record being read stable when background polling adds new samples.
const activeSample = shallowRef<QualitySample | null>(null)
const tooltipText = computed(() => activeIndex.value === null ? '' : title(activeSample.value))
const tooltipPosition = ref({ left: '0px', top: '0px' })
const tooltipReady = ref(false)
let positionSequence = 0
let hideTimer: ReturnType<typeof setTimeout> | undefined

function keepTooltipOpen() {
  clearTimeout(hideTimer)
  hideTimer = undefined
}

function scheduleHide() {
  keepTooltipOpen()
  // Allow crossing the small gap between a history cell and its floating panel.
  hideTimer = setTimeout(() => {
    if (strip.value?.matches(':hover, :focus-within') || tooltip.value?.matches(':hover, :focus-within')) return
    hideTooltip()
  }, 250)
}

function hideTooltip() {
  keepTooltipOpen()
  positionSequence++
  activeIndex.value = null
  activeSample.value = null
  tooltipReady.value = false
}

function selectSample(index: number) {
  keepTooltipOpen()
  const sample = history.value[index] ?? null
  if (activeIndex.value !== index || activeSample.value?.id !== sample?.id) {
    tooltip.value?.scrollTo(0, 0)
  }
  activeIndex.value = index
  activeSample.value = sample
}

function hoverSample(event: PointerEvent) {
  if (event.pointerType === 'touch' || !strip.value) return
  const bounds = strip.value.getBoundingClientRect()
  const gap = parseFloat(getComputedStyle(strip.value).columnGap) || 0
  // Include the narrow gaps in each cell's hit area, so crossing them does not
  // dismiss the tooltip. Fixed slot keys keep hover intact when records arrive.
  const index = Math.floor((event.clientX - bounds.left + gap / 2) / ((bounds.width + gap) / history.value.length))
  selectSample(Math.max(0, Math.min(history.value.length - 1, index)))
}

function focusSample() {
  keepTooltipOpen()
  if (activeIndex.value !== null) return
  selectSample(history.value.length - 1)
}

function navigateSample(event: KeyboardEvent) {
  if (event.key === 'Escape') { hideTooltip(); return }
  const direction = event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : 0
  if (!direction) return
  event.preventDefault()
  selectSample(Math.max(0, Math.min(history.value.length - 1, (activeIndex.value ?? history.value.length - 1) + direction)))
}

async function positionTooltip() {
  const sequence = ++positionSequence
  await nextTick()
  if (sequence !== positionSequence || activeIndex.value === null || !strip.value || !tooltip.value) return
  const cell = strip.value.children[activeIndex.value]
  if (!cell) { hideTooltip(); return }
  const anchor = cell.getBoundingClientRect()
  const box = tooltip.value.getBoundingClientRect()
  const padding = 8
  const left = Math.max(padding, Math.min(anchor.left + anchor.width / 2 - box.width / 2, window.innerWidth - box.width - padding))
  const above = anchor.top - box.height - padding
  const top = above >= padding ? above : Math.max(padding, Math.min(anchor.bottom + padding, window.innerHeight - box.height - padding))
  tooltipPosition.value = { left: `${left}px`, top: `${top}px` }
  tooltipReady.value = true
}

watch([activeIndex, tooltipText], () => void positionTooltip(), { flush: 'post' })
useEventListener(window, 'scroll', (event) => {
  if (event.target instanceof Node && tooltip.value?.contains(event.target)) return
  hideTooltip()
}, { capture: true, passive: true })
useEventListener(window, 'resize', hideTooltip)
useEventListener(window, 'blur', hideTooltip)
useEventListener(document, 'visibilitychange', () => { if (document.hidden) hideTooltip() })
onBeforeUnmount(keepTooltipOpen)
</script>
<template>
  <div class="space-y-3 border-t border-border/40 pt-3">
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 tabular-nums">
        <span class="font-medium text-muted-foreground">{{ t(`${p}.stripTitle`) }}</span>
        <slot name="controls" />
        <span :title="pausedByHealth ? t(`${p}.healthPausedHint`) : undefined" :class="selected && enabled && !pausedByHealth && state?.degraded ? 'text-red-600 dark:text-red-400' : status === 'normal' ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'">{{ t(`${p}.statuses.${status}`) }}</span>
        <template v-if="samples.length"><span class="text-muted-foreground">{{ t(`${p}.recent`, { count:samples.length }) }}</span><span :class="failed ? 'text-red-600 dark:text-red-400' : 'text-muted-foreground'">{{ t(`${p}.failedCount`, { count:failed }) }}</span><span :class="errors ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'">{{ t(`${p}.errorCount`, { count:errors }) }}</span><span class="text-emerald-700 dark:text-emerald-400" :title="t(`${p}.rateHint`)">{{ t(`${p}.passRate`, { rate }) }}</span></template>
      </div>
      <span v-if="state" class="text-muted-foreground" :title="title(state.latest)">{{ state.latest.model }} · {{ (state.latest.durationMs/1000).toFixed(2) }}s</span>
    </div>
    <div ref="strip" class="flex h-5 gap-px overflow-hidden rounded outline-none focus-visible:ring-2 focus-visible:ring-primary sm:gap-[2px]" role="group" tabindex="0" :aria-label="t(`${p}.historyLabel`, { count:samples.length, failed, errors, rate })" :aria-describedby="activeIndex !== null ? tooltipId : undefined" @pointerenter="hoverSample" @pointermove="hoverSample" @pointerleave="scheduleHide" @pointercancel="hideTooltip" @focus="focusSample" @blur="scheduleHide" @keydown="navigateSample">
      <span v-for="(sample,index) in history" :key="index" class="min-w-0 flex-1 rounded-[1px] transition-opacity" :class="[color(sample), activeIndex === index ? 'opacity-60' : '']" aria-hidden="true" />
    </div>
    <Teleport to="body">
      <div v-if="activeIndex !== null" :id="tooltipId" ref="tooltip" role="tooltip" tabindex="0" class="fixed z-[9999] max-h-[min(28rem,calc(100vh-1rem))] w-max max-w-[min(28rem,calc(100vw-1rem))] overflow-y-auto overscroll-contain whitespace-pre-wrap break-words rounded-lg border border-border bg-card px-3 py-2 text-xs leading-5 text-foreground shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :style="[tooltipPosition, { visibility: tooltipReady ? 'visible' : 'hidden' }]" @pointerenter="keepTooltipOpen" @pointerleave="scheduleHide" @focusin="keepTooltipOpen" @focusout="scheduleHide" @keydown.esc.stop.prevent="hideTooltip">{{ tooltipText }}</div>
    </Teleport>
    <p v-if="unavailable || (!selected && !manualOnly) || pausedByHealth || !samples.length" class="text-xs text-muted-foreground">{{ t(`${p}.${unavailable ? 'historyUnavailable' : pausedByHealth ? 'healthPausedHint' : !selected ? 'selectionHint' : enabled ? 'waiting' : 'enableHint'}`) }}</p>
    <p v-else-if="state?.latest.errorKey" class="text-xs text-amber-600 dark:text-amber-400">{{ t(connectionHealthMessageKey(state.latest.errorKey,te)) }}<span v-if="state.degraded"> · {{ t(`${p}.previousDegraded`) }}</span></p>
  </div>
</template>
