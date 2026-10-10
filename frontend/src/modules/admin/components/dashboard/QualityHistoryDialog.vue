<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useDocumentVisibility, useEventListener, useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { BrainCircuit, Code2, Eye, Loader2, Pause, RefreshCw, X } from 'lucide-vue-next'
import { getChannelQualityDetail, getChannelQualityHistory } from '../../api/connectionHealth'
import { connectionHealthMessageKey } from '../../composables/useConnectionHealth'
import type { QualityManualMethod, QualitySample } from '../../types/quality'
import { qualityHistoryNewestFirst, qualityPreviewDocument, qualitySampleMethod } from '../../utils/qualityHistory'

const props = defineProps<{ open: boolean; target: { targetId: string; name: string } | null }>()
const emit = defineEmits<{ close: [] }>()
const { t, te, locale } = useI18n()
const p = 'admin.connectionHealth.quality'
const d = `${p}.detail`
const dialog = ref<HTMLElement | null>(null)
const records = ref<QualitySample[]>([])
const filter = ref<QualityManualMethod | ''>('')
const methods: QualityManualMethod[] = ['questions', 'manxue_candy', 'manxue_pelican']
const selectedId = ref('')
const detail = ref<QualitySample | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const error = ref('')
const detailError = ref('')
const view = ref<'preview' | 'source'>('preview')
const previewEnabled = ref(true)
const visibleRecords = computed(() => qualityHistoryNewestFirst(records.value).filter(record => !filter.value || qualitySampleMethod(record) === filter.value))
const selected = computed(() => visibleRecords.value.find(record => record.id === selectedId.value))
const pelican = computed(() => detail.value && qualitySampleMethod(detail.value) === 'manxue_pelican')
const custom = computed(() => detail.value && qualitySampleMethod(detail.value) === 'questions')
const previewDocument = computed(() => detail.value?.html ? qualityPreviewDocument(detail.value.html) : '')
const date = (value?: string) => value ? new Date(value).toLocaleString(locale.value, { hour12: false }) : '—'
const methodName = (record: QualitySample) => t(`${p}.manualLabels.${qualitySampleMethod(record)}`)
const resultName = (record: QualitySample) => t(`${d}.results.${record.result}`)
const resultClass = (record: QualitySample) => record.result === 'passed' ? 'text-emerald-600 dark:text-emerald-400' : record.result === 'failed' ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400'
const message = (err: unknown) => t(connectionHealthMessageKey(err instanceof Error ? err.message : 'admin.connectionHealth.errors.request', te))
let listSequence = 0
let detailSequence = 0
let previousFocus: HTMLElement | null = null
let previousOverflow = ''

async function load() {
  if (!props.open || !props.target || loading.value) return
  const sequence = ++listSequence
  const target = props.target.targetId
  loading.value = true
  error.value = ''
  try {
    const result = await getChannelQualityHistory(target)
    if (sequence !== listSequence) return
    records.value = result
  } catch (err) { if (sequence === listSequence) error.value = message(err) }
  finally { if (sequence === listSequence) loading.value = false }
}
async function loadDetail() {
  const sequence = ++detailSequence
  detail.value = null
  detailError.value = ''
  detailLoading.value = false
  view.value = 'preview'
  previewEnabled.value = true
  if (!props.open || !props.target || !selectedId.value) return
  detailLoading.value = true
  try {
    const result = await getChannelQualityDetail(props.target.targetId, selectedId.value)
    if (sequence === detailSequence) detail.value = result
  } catch (err) { if (sequence === detailSequence) detailError.value = message(err) }
  finally { if (sequence === detailSequence) detailLoading.value = false }
}
watch(visibleRecords, list => {
  if (!list.some(record => record.id === selectedId.value)) selectedId.value = list[0]?.id || ''
})
watch(selectedId, () => void loadDetail())
watch(() => [props.open, props.target?.targetId] as const, async ([open], [wasOpen]) => {
  listSequence++; detailSequence++
  loading.value = false; detailLoading.value = false
  records.value = []; selectedId.value = ''; detail.value = null
  error.value = ''; detailError.value = ''; filter.value = ''
  if (!open) {
    document.body.style.overflow = previousOverflow
    await nextTick(); previousFocus?.focus()
    return
  }
  if (!wasOpen) {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  void load()
  await nextTick(); dialog.value?.focus()
})
onUnmounted(() => {
  listSequence++; detailSequence++
  if (props.open) { document.body.style.overflow = previousOverflow; previousFocus?.focus() }
})
const visibility = useDocumentVisibility()
useIntervalFn(() => { if (props.open && visibility.value === 'visible') void load() }, 30_000)
useEventListener(document, 'keydown', event => {
  if (!props.open || !dialog.value) return
  if (event.key === 'Escape') emit('close')
  if (event.key !== 'Tab') return
  const elements = Array.from(dialog.value.querySelectorAll<HTMLElement>('button:not(:disabled), select, [tabindex="0"]')).filter(el => el.offsetParent !== null)
  const first = elements[0], last = elements.at(-1)
  const outside = !dialog.value.contains(document.activeElement) || document.activeElement === dialog.value
  if (event.shiftKey && (outside || document.activeElement === first)) { event.preventDefault(); last?.focus() }
  else if (!event.shiftKey && (outside || document.activeElement === last)) { event.preventDefault(); first?.focus() }
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-2 sm:p-5" @click.self="emit('close')">
      <section ref="dialog" role="dialog" aria-modal="true" aria-labelledby="quality-detail-title" tabindex="-1" class="flex h-[94dvh] w-full max-w-[1440px] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-2xl outline-none">
        <header class="flex shrink-0 items-start justify-between gap-3 border-b border-border px-4 py-4 sm:px-5">
          <div class="min-w-0">
            <h2 id="quality-detail-title" class="flex items-center gap-2 text-base font-semibold"><BrainCircuit class="h-4 w-4 shrink-0 text-primary" /><span class="truncate">{{ t(`${d}.title`, { name: target?.name || '' }) }}</span></h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t(`${d}.subtitle`) }}</p>
          </div>
          <button type="button" class="quality-detail-button shrink-0" :aria-label="t(`${p}.close`)" @click="emit('close')"><X class="h-4 w-4" /></button>
        </header>
        <div class="flex min-h-0 flex-1 flex-col md:flex-row">
          <aside class="flex max-h-[32vh] shrink-0 flex-col border-b border-border md:max-h-none md:w-64 md:border-b-0 md:border-r">
            <div class="flex items-center gap-2 border-b border-border/50 p-3">
              <select v-model="filter" :aria-label="t(`${d}.filter`)" class="min-w-0 flex-1 rounded-md border border-border bg-card px-2 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-primary">
                <option value="">{{ t(`${d}.allTypes`) }}</option>
                <option v-for="method in methods" :key="method" :value="method">{{ t(`${p}.manualLabels.${method}`) }}</option>
              </select>
              <button type="button" class="quality-detail-button" :disabled="loading" :aria-label="t(`${d}.refresh`)" @click="load"><RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" /></button>
            </div>
            <div v-if="error" role="alert" class="p-3 text-xs text-destructive">{{ error }} <button class="underline" @click="load">{{ t(`${p}.retry`) }}</button></div>
            <p v-if="loading && !records.length" role="status" class="p-4 text-xs text-muted-foreground">{{ t(`${p}.loading`) }}</p>
            <p v-else-if="!visibleRecords.length && !error" class="p-4 text-xs text-muted-foreground">{{ t(`${d}.empty`) }}</p>
            <nav class="min-h-0 flex-1 space-y-2 overflow-y-auto p-3" :aria-label="t(`${d}.records`)">
              <button v-for="record in visibleRecords" :key="record.id" type="button" class="block w-full rounded-lg border p-3 text-left text-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary" :class="selectedId === record.id ? 'border-primary bg-primary/5' : 'border-border hover:bg-surface/70'" :aria-current="selectedId === record.id ? 'true' : undefined" @click="selectedId = record.id">
                <time class="block font-semibold text-foreground" :datetime="record.startedAt || record.createdAt">{{ date(record.startedAt || record.createdAt) }}</time>
                <span class="mt-1.5 flex flex-wrap items-center gap-1.5"><span class="rounded bg-surface px-1.5 py-0.5 text-muted-foreground">{{ methodName(record) }}</span><span :class="resultClass(record)">{{ resultName(record) }}</span></span>
                <span class="mt-1.5 block break-all text-muted-foreground">{{ record.model }} · {{ (record.durationMs / 1000).toFixed(2) }}s</span>
              </button>
            </nav>
            <p class="shrink-0 border-t border-border/50 px-3 py-2 text-xs text-muted-foreground">{{ t(`${d}.count`, { count: visibleRecords.length }) }}</p>
          </aside>
          <main class="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-5">
            <div v-if="detailLoading" role="status" class="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground"><Loader2 class="h-4 w-4 animate-spin" />{{ t(`${p}.loading`) }}</div>
            <div v-else-if="detailError" role="alert" class="rounded-lg bg-destructive/5 p-4 text-sm text-destructive">{{ detailError }} <button class="underline" @click="loadDetail">{{ t(`${p}.retry`) }}</button></div>
            <article v-else-if="detail && selected" class="space-y-4">
              <div class="space-y-3 border-b border-border pb-4">
                <p class="flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground"><span>{{ t(`${d}.started`) }} {{ date(detail.startedAt) }}</span><span>{{ t(`${d}.finished`) }} {{ date(detail.createdAt) }}</span></p>
                <div class="flex flex-wrap items-center gap-2 text-xs"><span class="rounded-md border border-primary/40 bg-primary/5 px-2 py-1 text-primary">{{ detail.model }}</span><span class="rounded-md bg-surface px-2 py-1">{{ methodName(detail) }}</span><strong :class="resultClass(detail)">{{ resultName(detail) }}</strong><span class="text-muted-foreground">{{ t(`${d}.duration`) }} {{ (detail.durationMs / 1000).toFixed(2) }}s</span></div>
              </div>
              <p v-if="detail.errorKey" role="status" class="rounded-lg bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-400">{{ t(connectionHealthMessageKey(detail.errorKey, te)) }}</p>
              <section v-if="detail.report" class="rounded-lg bg-surface/50 p-3"><h3 class="mb-2 text-xs font-medium text-muted-foreground">{{ t(`${d}.assessment`) }}</h3><p class="whitespace-pre-wrap break-words text-sm">{{ detail.report }}</p></section>
              <template v-if="pelican">
                <div v-if="detail.html" class="space-y-3">
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="quality-detail-button" :class="view === 'preview' ? 'text-primary' : ''" :aria-pressed="view === 'preview'" @click="view = 'preview'; previewEnabled = true"><Eye class="h-3.5 w-3.5" />{{ t(`${d}.preview`) }}</button>
                    <button type="button" class="quality-detail-button" :aria-pressed="view === 'source'" @click="view = 'source'"><Code2 class="h-3.5 w-3.5" />{{ t(`${d}.source`) }}</button>
                    <button v-if="view === 'preview' && previewEnabled" type="button" class="quality-detail-button" @click="previewEnabled = false"><Pause class="h-3.5 w-3.5" />{{ t(`${d}.stopPreview`) }}</button>
                  </div>
                  <iframe v-if="view === 'preview' && previewEnabled" :key="detail.id" :srcdoc="previewDocument" sandbox="allow-scripts" referrerpolicy="no-referrer" tabindex="-1" :title="t(`${d}.previewTitle`)" class="h-[58vh] min-h-80 w-full rounded-lg border border-border bg-white" />
                  <p v-else-if="view === 'preview'" class="rounded-lg bg-surface p-6 text-center text-sm text-muted-foreground">{{ t(`${d}.previewStopped`) }}</p>
                  <pre v-else tabindex="0" class="max-h-[60vh] overflow-auto whitespace-pre rounded-lg bg-surface p-4 text-xs leading-5">{{ detail.html }}</pre>
                </div>
                <p v-else class="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">{{ t(`${d}.${detail.htmlTooLarge ? 'htmlTooLarge' : 'noHtml'}`) }}</p>
              </template>
              <template v-else>
                <section v-if="custom" class="rounded-lg border border-border p-4"><h3 class="mb-2 text-sm font-medium">{{ detail.questionName || t(`${d}.question`) }}</h3><p class="whitespace-pre-wrap break-words text-sm leading-6 text-muted-foreground">{{ detail.prompt || t(`${d}.noPrompt`) }}</p></section>
                <div v-if="custom" class="grid gap-3 sm:grid-cols-2">
                  <section class="rounded-lg bg-surface/60 p-4"><h3 class="mb-2 text-xs font-medium text-muted-foreground">{{ t(`${p}.expectedAnswer`) }}</h3><pre class="whitespace-pre-wrap break-words text-sm">{{ detail.expectedAnswer || '—' }}</pre><p class="mt-3 text-xs text-muted-foreground">{{ t(`${d}.matchMode`) }} {{ t(`${p}.matchModes.${detail.matchMode}`) }}</p></section>
                  <section class="space-y-2 rounded-lg bg-surface/60 p-4 text-xs"><h3 class="font-medium text-muted-foreground">{{ t(`${d}.judgement`) }}</h3><p v-if="detail.mode">{{ t(`${p}.modes.${detail.mode}`) }}</p><p>{{ t(`${p}.contentResult`) }} · {{ detail.result === 'error' ? '—' : t(`${p}.${detail.contentPassed ? 'passed' : 'failed'}`) }}</p><p>{{ t(`${p}.timeResult`) }} · {{ detail.result === 'error' ? '—' : t(`${p}.${detail.timePassed ? 'passed' : 'failed'}`) }} · &lt; {{ (detail.maxDurationMs / 1000).toFixed(2) }}s</p></section>
                </div>
                <section class="rounded-lg border border-border p-4"><h3 class="mb-3 text-sm font-medium">{{ t(`${p}.actualAnswer`) }}</h3><pre class="whitespace-pre-wrap break-words text-sm leading-6 text-muted-foreground">{{ detail.answer || t(`${d}.noAnswer`) }}</pre></section>
                <p v-if="!custom" class="text-xs text-muted-foreground">{{ t(`${d}.candyHint`) }}</p>
              </template>
            </article>
            <div v-else class="flex h-full items-center justify-center text-sm text-muted-foreground">{{ t(`${d}.selectRecord`) }}</div>
          </main>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.quality-detail-button { @apply inline-flex items-center justify-center gap-1.5 rounded-md border border-border px-2 py-1.5 text-xs text-muted-foreground outline-none hover:bg-surface hover:text-primary focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40; }
</style>
