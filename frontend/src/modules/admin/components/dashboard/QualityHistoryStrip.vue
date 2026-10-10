<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, useId, watch } from 'vue'
import { useElementSize, useEventListener } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { connectionHealthMessageKey, formatConnectionHealthTime } from '../../composables/useConnectionHealth'
import type { QualitySample, QualityState } from '../../types/quality'
const props = defineProps<{ samples?: QualitySample[]; state?: QualityState; enabled?: boolean; pausedByHealth?: boolean; suspendedByQuality?: boolean; selected?: boolean; unavailable?: boolean }>()
const { t, te } = useI18n()
const p = 'admin.connectionHealth.quality'
const samples = computed(() => [...(props.samples || [])].sort((a,b) => Date.parse(b.createdAt)-Date.parse(a.createdAt) || b.id.localeCompare(a.id)).slice(0,100))
const history = computed(() => samples.value.slice().reverse())
const passed = computed(() => samples.value.filter(s => s.result === 'passed').length)
const failed = computed(() => samples.value.filter(s => s.result === 'failed').length)
const errors = computed(() => samples.value.filter(s => s.result === 'error').length)
const rate = computed(() => passed.value + failed.value ? `${(passed.value/(passed.value+failed.value)*100).toFixed(1)}%` : '—')
const manualOnly = computed(() => (!props.selected || !props.enabled) && samples.value.some(sample => sample.manual))
const status = computed(() => props.unavailable ? 'unavailable' : props.pausedByHealth ? 'healthPaused' : props.suspendedByQuality ? 'qualitySuspended' : manualOnly.value ? 'manualOnly' : !props.selected ? 'notSelected' : !props.enabled ? 'off' : props.state?.status || 'pending')
const color = (sample: QualitySample) => sample.result === 'passed' ? 'text-emerald-500 dark:text-emerald-400' : sample.result === 'failed' ? 'text-red-500 dark:text-red-400' : 'text-amber-500 dark:text-amber-400'
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
const { width: stripWidth } = useElementSize(strip)
const chartHeight = 52
const chartPadding = 8
const plotWidth = computed(() => Math.max(stripWidth.value, chartPadding * 2 + 1))
const points = computed(() => history.value.map((sample, index) => ({
  sample,
  x: chartPadding + (plotWidth.value - chartPadding * 2) * (history.value.length > 1 ? index / (history.value.length - 1) : 1),
  y: sample.result === 'passed' ? 12 : sample.result === 'failed' ? 40 : 26,
})))
const segments = computed(() => points.value.slice(1).map((point, index) => ({ from: points.value[index]!, to: point })))
const pointRadius = computed(() => Math.max(1.7, Math.min(3, (plotWidth.value - chartPadding * 2) / Math.max(1, history.value.length - 1) * 0.36)))
const tooltip = ref<HTMLElement | null>(null)
const tooltipId = useId()
const activeIndex = ref<number | null>(null)
// Keep the record being read stable when background polling adds new samples.
const activeSample = shallowRef<QualitySample | null>(null)
const activePoint = computed(() => points.value.find(point => point.sample.id === activeSample.value?.id))
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
  if (!history.value.length) return
  keepTooltipOpen()
  const sample = history.value[index] ?? null
  if (activeIndex.value !== index || activeSample.value?.id !== sample?.id) {
    tooltip.value?.scrollTo(0, 0)
  }
  activeIndex.value = index
  activeSample.value = sample
}

function selectAt(clientX: number) {
  if (!strip.value || !history.value.length) return
  const bounds = strip.value.getBoundingClientRect()
  // Select the nearest result across the full chart height, including the gaps.
  const index = Math.round((clientX - bounds.left - chartPadding) / Math.max(1, bounds.width - chartPadding * 2) * (history.value.length - 1))
  selectSample(Math.max(0, Math.min(history.value.length - 1, index)))
}

function hoverSample(event: PointerEvent) {
  if (event.pointerType !== 'touch') selectAt(event.clientX)
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
  const anchor = strip.value.getBoundingClientRect()
  const anchorX = anchor.left + (activePoint.value?.x ?? anchor.width - chartPadding)
  const box = tooltip.value.getBoundingClientRect()
  const padding = 8
  const left = Math.max(padding, Math.min(anchorX - box.width / 2, window.innerWidth - box.width - padding))
  const above = anchor.top - box.height - padding
  const top = above >= padding ? above : Math.max(padding, Math.min(anchor.bottom + padding, window.innerHeight - box.height - padding))
  tooltipPosition.value = { left: `${left}px`, top: `${top}px` }
  tooltipReady.value = true
}

watch([activeIndex, tooltipText, stripWidth], () => void positionTooltip(), { flush: 'post' })
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
  <div class="space-y-2 border-t border-border/40 pt-3">
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 tabular-nums">
        <span class="font-medium text-muted-foreground">{{ t(`${p}.stripTitle`) }}</span>
        <slot name="controls" />
        <span :title="pausedByHealth ? t(`${p}.healthPausedHint`) : undefined" :class="suspendedByQuality || (selected && enabled && !pausedByHealth && state?.degraded) ? 'text-red-600 dark:text-red-400' : status === 'normal' ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'">{{ t(`${p}.statuses.${status}`) }}</span>
      </div>
      <span v-if="state" class="text-muted-foreground" :title="title(state.latest)">{{ state.latest.model }} · {{ (state.latest.durationMs/1000).toFixed(2) }}s</span>
    </div>
    <div ref="strip" class="relative h-[52px] rounded-md bg-emerald-500/[0.035] outline-none focus-visible:ring-2 focus-visible:ring-primary dark:bg-emerald-400/[0.04]" role="group" :tabindex="samples.length ? 0 : -1" :aria-label="t(`${p}.historyLabel`, { count:samples.length, failed, errors, rate })" :aria-describedby="activeIndex !== null ? tooltipId : undefined" @pointerenter="hoverSample" @pointermove="hoverSample" @pointerleave="scheduleHide" @pointercancel="hideTooltip" @click="selectAt($event.clientX)" @focus="focusSample" @blur="scheduleHide" @keydown="navigateSample">
      <svg class="block h-full w-full overflow-hidden" :viewBox="`0 0 ${plotWidth} ${chartHeight}`" aria-hidden="true">
        <line :x1="chartPadding" :x2="plotWidth-chartPadding" y1="12" y2="12" class="stroke-emerald-500/10 dark:stroke-emerald-400/10" stroke-dasharray="2 4" />
        <line :x1="chartPadding" :x2="plotWidth-chartPadding" y1="40" y2="40" class="stroke-red-500/10 dark:stroke-red-400/10" stroke-dasharray="2 4" />
        <line v-for="segment in segments" :key="segment.to.sample.id" :x1="segment.from.x" :y1="segment.from.y" :x2="segment.to.x" :y2="segment.to.y" stroke-width="1" :class="segment.from.sample.result === 'error' || segment.to.sample.result === 'error' ? 'stroke-amber-500/35 dark:stroke-amber-400/35' : 'stroke-emerald-500/35 dark:stroke-emerald-400/35'" />
        <circle v-for="point in points" :key="point.sample.id" :cx="point.x" :cy="point.y" :r="pointRadius" fill="currentColor" :class="[color(point.sample), 'stroke-card']" stroke-width="0.8" />
        <circle v-if="activePoint" :cx="activePoint.x" :cy="activePoint.y" :r="pointRadius+2" fill="none" stroke="currentColor" stroke-width="1" :class="color(activePoint.sample)" />
      </svg>
      <span v-if="!samples.length" class="pointer-events-none absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">{{ t(`${p}.noRecord`) }}</span>
    </div>
    <div v-if="samples.length" class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs tabular-nums">
      <span class="font-medium text-primary" :title="t(`${p}.rateHint`)">{{ t(`${p}.passRateShort`, { rate }) }}</span>
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span v-if="errors" class="text-amber-600 dark:text-amber-400">{{ t(`${p}.errorCount`, { count:errors }) }}</span>
        <span class="text-muted-foreground" :title="t(`${p}.rateHint`)">{{ t(`${p}.passedRatio`, { passed, total:passed+failed }) }}</span>
      </div>
    </div>
    <Teleport to="body">
      <div v-if="activeIndex !== null" :id="tooltipId" ref="tooltip" role="tooltip" tabindex="0" class="fixed z-[9999] max-h-[min(28rem,calc(100vh-1rem))] w-max max-w-[min(28rem,calc(100vw-1rem))] overflow-y-auto overscroll-contain whitespace-pre-wrap break-words rounded-lg border border-border bg-card px-3 py-2 text-xs leading-5 text-foreground shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :style="[tooltipPosition, { visibility: tooltipReady ? 'visible' : 'hidden' }]" @pointerenter="keepTooltipOpen" @pointerleave="scheduleHide" @focusin="keepTooltipOpen" @focusout="scheduleHide" @keydown.esc.stop.prevent="hideTooltip">{{ tooltipText }}</div>
    </Teleport>
    <p v-if="unavailable || (!selected && !manualOnly) || pausedByHealth || !samples.length" class="text-xs text-muted-foreground">{{ t(`${p}.${unavailable ? 'historyUnavailable' : pausedByHealth ? 'healthPausedHint' : !selected ? 'selectionHint' : enabled ? 'waiting' : 'enableHint'}`) }}</p>
    <p v-else-if="state?.latest.errorKey" class="text-xs text-amber-600 dark:text-amber-400">{{ t(connectionHealthMessageKey(state.latest.errorKey,te)) }}<span v-if="state.degraded"> · {{ t(`${p}.previousDegraded`) }}</span></p>
  </div>
</template>
