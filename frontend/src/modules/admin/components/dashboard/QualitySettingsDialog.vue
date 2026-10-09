<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowUp, BrainCircuit, Loader2, Plus, Save, Trash2, X } from 'lucide-vue-next'
import { getQualitySettings, saveQualitySettings } from '../../api/connectionHealth'
import { connectionHealthMessageKey } from '../../composables/useConnectionHealth'
import type { QualityQuestion, QualitySettings } from '../../types/quality'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t, te } = useI18n()
const p = 'admin.connectionHealth.quality'
const form = ref<QualitySettings | null>(null)
const busy = ref(false)
const error = ref('')
const tab = ref<'questions' | 'schedule'>('questions')
const selected = ref('')
const question = computed(() => form.value?.questions.find(q => q.id === selected.value))
const dialog = ref<HTMLElement | null>(null)
let sequence = 0
let previousFocus: HTMLElement | null = null
const readError = (err: unknown) => err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
const close = () => { if (!busy.value) emit('close') }
async function load() {
  const current = ++sequence
  busy.value = true; error.value = ''
  try { const result = await getQualitySettings(); if (current !== sequence) return; form.value = result; selected.value = result.questions[0]?.id || '' }
  catch (err) { if (current === sequence) error.value = readError(err) }
  finally { if (current === sequence) busy.value = false }
}
watch(() => props.open, async open => {
  sequence++
  if (!open) { previousFocus?.focus(); return }
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  form.value = null; tab.value = 'questions'; void load()
  await nextTick(); dialog.value?.focus()
})
onUnmounted(() => { sequence++; previousFocus?.focus() })
useEventListener(document, 'keydown', event => {
  if (!props.open || !dialog.value) return
  if (event.key === 'Escape') close()
  if (event.key !== 'Tab') return
  const elements = Array.from(dialog.value.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled)')).filter(element => element.offsetParent !== null)
  const first = elements[0], last = elements.at(-1)
  const outside = !dialog.value.contains(document.activeElement) || document.activeElement === dialog.value
  if (event.shiftKey && (outside || document.activeElement === first)) { event.preventDefault(); last?.focus() }
  else if (!event.shiftKey && (outside || document.activeElement === last)) { event.preventDefault(); first?.focus() }
})
function addQuestion() {
  if (!form.value || form.value.questions.length >= 50) return
  const q: QualityQuestion = { id: crypto.randomUUID?.() || `q-${Date.now()}-${Math.random().toString(16).slice(2)}`, name: t(`${p}.newQuestion`), enabled: true, prompt: '', answer: '', matchMode: 'answer', maxDurationMs: 20000 }
  form.value.questions.push(q); selected.value = q.id
}
function removeQuestion(index: number) {
  form.value?.questions.splice(index, 1)
  if (!question.value) selected.value = form.value?.questions[Math.max(0, index - 1)]?.id || ''
}
function move(index: number, offset: number) {
  const list = form.value?.questions
  if (!list || index + offset < 0 || index + offset >= list.length) return
  const q = list.splice(index, 1)[0]; if (q) list.splice(index + offset, 0, q)
}
async function save() {
  if (!form.value || busy.value) return
  // Validate every question, including those outside the current editor.
  const invalid = form.value.questions.find(q => !q.name.trim() || !q.prompt.trim() || (form.value?.mode !== 'time' && !q.answer.trim()))
  if (invalid) { tab.value = 'questions'; selected.value = invalid.id; error.value = `${p}.invalidQuestion`; return }
  const current = sequence; busy.value = true; error.value = ''
  try { await saveQualitySettings(form.value); emit('saved'); if (current === sequence) emit('close') }
  catch (err) { if (current === sequence) error.value = readError(err) }
  finally { if (current === sequence) busy.value = false }
}
async function importConfig(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]; if (!file || !form.value) return
  try {
    if (file.size > 1024 * 1024) throw new Error(`${p}.importFailed`)
    const raw = JSON.parse(await file.text()); const data = raw.settings || raw
    if (!Array.isArray(data.questions)) throw new Error(`${p}.importFailed`)
    // Import Qerkai's question and scheduling fields only. Never import actions.
    const numeric: Record<string, keyof QualitySettings> = { interval_seconds: 'intervalSeconds', retry_seconds: 'retrySeconds', failure_limit: 'failureLimit', recovery_limit: 'recoveryLimit', concurrency: 'concurrency', timeout_seconds: 'timeoutSeconds', history_limit: 'historyLimit' }
    for (const [key, target] of Object.entries(numeric)) if (typeof data[key] === 'number') Object.assign(form.value, { [target]: data[key] })
    if (typeof data.model === 'string') form.value.model = data.model
    if (['low', 'medium', 'high', 'xhigh'].includes(data.reasoning_effort)) form.value.reasoningEffort = data.reasoning_effort
    if (['content', 'time', 'content_time'].includes(data.mode)) form.value.mode = data.mode
    form.value.questions = data.questions.slice(0, 50).map((q: Record<string, unknown>, i: number) => ({ id: typeof q.id === 'string' ? q.id : `import-${i}`, name: String(q.name || ''), enabled: q.enabled === true, prompt: String(q.prompt || ''), answer: String(q.answer || ''), matchMode: String(q.match_mode || q.matchMode || 'answer') as QualityQuestion['matchMode'], maxDurationMs: Number(q.max_duration_ms ?? q.maxDurationMs ?? 20000) }))
    form.value.enabled = false; selected.value = form.value.questions[0]?.id || ''; error.value = ''; tab.value = 'questions'
  } catch (err) { error.value = err instanceof Error && err.message.startsWith(p) ? err.message : `${p}.importFailed` }
  input.value = ''
}
const numbers: { key: 'intervalSeconds' | 'retrySeconds' | 'failureLimit' | 'recoveryLimit' | 'concurrency' | 'timeoutSeconds' | 'maxTokens' | 'historyLimit'; min: number; max: number }[] = [
  { key: 'intervalSeconds', min: 10, max: 86400 }, { key: 'retrySeconds', min: 10, max: 86400 },
  { key: 'failureLimit', min: 1, max: 20 }, { key: 'recoveryLimit', min: 1, max: 20 },
  { key: 'concurrency', min: 1, max: 32 }, { key: 'timeoutSeconds', min: 5, max: 300 },
  { key: 'maxTokens', min: 128, max: 32768 }, { key: 'historyLimit', min: 1, max: 1000 },
]
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-[150] flex items-center justify-center bg-black/40 p-3 sm:p-6" @click.self="close">
      <section ref="dialog" role="dialog" aria-modal="true" tabindex="-1" :aria-label="t(`${p}.settingsTitle`)" class="flex max-h-[calc(100dvh-2rem)] w-full max-w-4xl flex-col overflow-hidden rounded-xl border border-border bg-card text-card-foreground shadow-xl outline-none">
        <header class="flex items-start justify-between gap-3 border-b border-border p-5">
          <div><h2 class="flex items-center gap-2 font-semibold"><BrainCircuit class="h-5 w-5 text-primary" />{{ t(`${p}.settingsTitle`) }}</h2><p class="mt-2 text-xs leading-5 text-muted-foreground">{{ t(`${p}.settingsHint`) }}</p></div>
          <button type="button" :disabled="busy" :aria-label="t(`${p}.close`)" class="rounded p-1 hover:bg-surface disabled:opacity-40" @click="close"><X class="h-5 w-5" /></button>
        </header>
        <p v-if="busy && !form" class="flex items-center gap-2 p-5 text-sm"><Loader2 class="h-4 w-4 animate-spin" />{{ t(`${p}.loading`) }}</p>
        <form v-if="form" id="quality-settings-form" novalidate class="min-h-0 flex-1 overflow-y-auto p-5" @submit.prevent="save">
          <fieldset :disabled="busy" class="space-y-5 disabled:opacity-60">
            <label class="flex items-center gap-2 text-sm font-medium"><input v-model="form.enabled" type="checkbox" class="h-4 w-4 accent-primary">{{ t(`${p}.globalEnabled`) }}</label>
            <nav class="flex gap-5 border-b border-border" :aria-label="t(`${p}.settingsTitle`)"><button v-for="item in ['questions', 'schedule'] as const" :key="item" type="button" class="border-b-2 pb-3 text-sm" :class="tab === item ? 'border-primary text-primary' : 'border-transparent text-muted-foreground'" @click="tab = item">{{ t(`${p}.${item}`) }}</button></nav>
            <div v-show="tab === 'questions'" class="space-y-5">
              <div class="grid gap-4 sm:grid-cols-3">
                <label class="quality-label">{{ t(`${p}.model`) }}<input v-model="form.model" class="quality-input" maxlength="200" required></label>
                <label class="quality-label">{{ t(`${p}.reasoningEffort`) }}<select v-model="form.reasoningEffort" class="quality-input"><option value="">{{ t(`${p}.defaultEffort`) }}</option><option v-for="effort in ['low','medium','high','xhigh']" :key="effort" :value="effort">{{ effort }}</option></select></label>
                <label class="quality-label">{{ t(`${p}.mode`) }}<select v-model="form.mode" class="quality-input"><option v-for="mode in ['content_time','content','time']" :key="mode" :value="mode">{{ t(`${p}.modes.${mode}`) }}</option></select></label>
              </div>
              <div class="flex items-center justify-between gap-2"><h3 class="text-sm font-medium">{{ t(`${p}.questionBank`) }} <span class="text-muted-foreground">{{ form.questions.length }}/50</span></h3><button type="button" :disabled="form.questions.length >= 50" class="inline-flex items-center gap-1 rounded-md border border-border px-3 py-1.5 text-xs disabled:opacity-40" @click="addQuestion"><Plus class="h-3.5 w-3.5" />{{ t(`${p}.add`) }}</button></div>
              <div class="grid gap-4 md:grid-cols-[14rem_minmax(0,1fr)]">
                <div class="space-y-1">
                  <div v-for="(q, index) in form.questions" :key="q.id" class="flex items-center gap-2 rounded-lg border px-2 py-2" :class="selected === q.id ? 'border-primary/30 bg-primary/5' : 'border-border/60'">
                    <input v-model="q.enabled" type="checkbox" :aria-label="t(`${p}.enableQuestion`, { name: q.name })" class="accent-primary">
                    <button type="button" class="min-w-0 flex-1 truncate text-left text-xs" @click="selected = q.id">{{ q.name }}</button>
                    <button type="button" :disabled="index === 0" :aria-label="t(`${p}.moveUp`)" class="text-muted-foreground disabled:opacity-20" @click="move(index,-1)"><ArrowUp class="h-3 w-3" /></button>
                    <button type="button" :disabled="index === form.questions.length-1" :aria-label="t(`${p}.moveDown`)" class="text-muted-foreground disabled:opacity-20" @click="move(index,1)"><ArrowDown class="h-3 w-3" /></button>
                    <button type="button" :aria-label="t(`${p}.removeQuestion`, { name: q.name })" class="text-muted-foreground hover:text-destructive" @click="removeQuestion(index)"><Trash2 class="h-3.5 w-3.5" /></button>
                  </div>
                </div>
                <div v-if="question" class="space-y-4 rounded-lg border border-border p-4">
                  <div class="grid gap-4 sm:grid-cols-2"><label class="quality-label">{{ t(`${p}.questionName`) }}<input v-model="question.name" class="quality-input" maxlength="60"></label><label class="quality-label">{{ t(`${p}.maxDurationMs`) }}<input v-model.number="question.maxDurationMs" class="quality-input" type="number" min="1" max="300000" required></label></div>
                  <label class="quality-label">{{ t(`${p}.prompt`) }}<textarea v-model="question.prompt" class="quality-input min-h-32 resize-y" rows="5" maxlength="15000" /></label>
                  <label class="quality-label">{{ t(`${p}.matchMode`) }}<select v-model="question.matchMode" class="quality-input"><option v-for="mode in ['answer','keyword','regex']" :key="mode" :value="mode">{{ t(`${p}.matchModes.${mode}`) }}</option></select></label>
                  <label class="quality-label">{{ t(`${p}.answer`) }}<textarea v-model="question.answer" class="quality-input resize-y" rows="2" maxlength="2000" /></label>
                  <p class="text-xs leading-5 text-muted-foreground">{{ t(`${p}.answerHint`) }}</p>
                </div>
              </div>
            </div>
            <div v-show="tab === 'schedule'" class="space-y-5">
              <div class="grid gap-4 sm:grid-cols-2"><label v-for="item in numbers" :key="item.key" class="quality-label">{{ t(`${p}.${item.key}`) }}<input v-model.number="form[item.key]" type="number" :min="item.min" :max="item.max" step="1" class="quality-input" required></label></div>
              <p class="text-xs leading-5 text-muted-foreground">{{ t(`${p}.scheduleHint`) }}</p>
              <label class="quality-label">{{ t(`${p}.importConfig`) }}<input type="file" accept="application/json,.json" class="text-xs" @change="importConfig"><span class="text-xs font-normal text-muted-foreground">{{ t(`${p}.importHint`) }}</span></label>
            </div>
          </fieldset>
        </form>
        <footer class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-border p-4">
          <p v-if="error" role="alert" class="max-w-xl text-sm text-destructive">{{ t(connectionHealthMessageKey(error,te)) }}</p><span v-else class="text-xs text-muted-foreground">{{ t(`${p}.displayOnly`) }}</span>
          <button v-if="!form && !busy" type="button" class="text-sm text-primary" @click="load">{{ t(`${p}.retry`) }}</button>
          <button v-if="form" type="submit" form="quality-settings-form" :disabled="busy" class="ml-auto inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"><Loader2 v-if="busy" class="h-4 w-4 animate-spin" /><Save v-else class="h-4 w-4" />{{ t(`${p}.${busy ? 'saving' : 'save'}`) }}</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.quality-label { @apply flex flex-col gap-2 text-xs font-medium; }
.quality-input { @apply w-full rounded-lg border border-border bg-background px-3 py-2 text-sm font-normal text-foreground outline-none focus:ring-2 focus:ring-primary/30; }
</style>
