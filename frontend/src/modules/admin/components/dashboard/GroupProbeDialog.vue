<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { Loader2, X, Zap } from 'lucide-vue-next'
import { prepareGroupProbe, probeAdminGroup } from '../../api/connectionHealth'
import { connectionHealthMessageKey } from '../../composables/useConnectionHealth'
import type { AdminGroupHealth, GroupProbeSample, ManualProbeModelOption } from '../../types/connectionHealth'

const props = defineProps<{ group: AdminGroupHealth | null; open: boolean }>()
const emit = defineEmits<{ close: []; probed: [] }>()
const { t, te } = useI18n()
const prefix = 'admin.connectionHealth.groupProbe'
const preparing = ref(false)
const probing = ref(false)
const prepared = ref(false)
const models = ref<ManualProbeModelOption[]>([])
const model = ref('')
const unavailableModels = ref(false)
const error = ref('')
const result = ref<GroupProbeSample | null>(null)
let sequence = 0
const dialog = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
const restoreFocus = () => previousFocus?.focus()
onUnmounted(() => { sequence++; restoreFocus() })
const trapFocus = (event: KeyboardEvent) => {
  if (event.key !== 'Tab' || !dialog.value) return
  const elements = Array.from(dialog.value.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)'))
  const first = elements[0]
  const last = elements.at(-1)
  if (!first) { event.preventDefault(); return }
  const outside = !dialog.value.contains(document.activeElement)
  if (event.shiftKey && (outside || document.activeElement === first || document.activeElement === dialog.value)) {
    event.preventDefault(); last?.focus()
  } else if (!event.shiftKey && (outside || document.activeElement === last || document.activeElement === dialog.value)) {
    event.preventDefault(); first.focus()
  }
}
const busy = computed(() => preparing.value || probing.value)
const readable = (key: string) => t(connectionHealthMessageKey(key, te))
const prepare = async () => {
  if (!props.group) return
  const group = props.group
  const current = ++sequence
  preparing.value = true
  prepared.value = false
  error.value = ''
  try {
    const response = await prepareGroupProbe(group.id)
    if (current !== sequence) return
    models.value = response.models
    unavailableModels.value = response.modelListUnavailable || !response.models.length
    model.value = group.recentProbes?.[0]?.modelName || response.models[0]?.id || ''
    prepared.value = true
  } catch (err) {
    if (current === sequence) error.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
  } finally {
    if (current === sequence) preparing.value = false
  }
}
watch(() => [props.open, props.group?.id], async () => {
  sequence++
  preparing.value = false
  probing.value = false
  if (!props.open || !props.group) { restoreFocus(); return }
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  result.value = null
  models.value = []
  model.value = ''
  void prepare()
  await nextTick()
  dialog.value?.focus()
})
const probe = async () => {
  if (!props.group || !model.value.trim() || busy.value || !prepared.value) return
  const groupId = props.group.id
  const current = sequence
  probing.value = true
  error.value = ''
  result.value = null
  try {
    const response = await probeAdminGroup(groupId, model.value.trim())
    if (current === sequence) result.value = response
    emit('probed')
  } catch (err) {
    if (current === sequence) error.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
  } finally {
    if (current === sequence) probing.value = false
  }
}
const close = () => { if (!busy.value) emit('close') }
useEventListener(document, 'keydown', event => {
  if (!props.open) return
  if (event.key === 'Escape') close()
  trapFocus(event)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open && group" class="fixed inset-0 z-[150] flex items-center justify-center bg-black/40 p-4" @click.self="close">
      <section ref="dialog" role="dialog" aria-modal="true" tabindex="-1" :aria-label="t(`${prefix}.title`)" class="w-full max-w-md space-y-5 rounded-xl border border-border bg-card p-5 text-card-foreground shadow-xl outline-none">
        <header class="flex items-center justify-between gap-3">
          <div class="min-w-0"><h2 class="font-semibold">{{ t(`${prefix}.title`) }}</h2><p class="mt-1 truncate text-sm text-muted-foreground">{{ group.name }} · {{ group.platform }}</p></div>
          <button type="button" :disabled="busy" :aria-label="t(`${prefix}.close`)" class="rounded p-1 text-muted-foreground hover:bg-surface disabled:opacity-40" @click="close"><X class="h-4 w-4" /></button>
        </header>
        <p class="text-sm leading-6 text-muted-foreground">{{ t(`${prefix}.hint`) }}</p>
        <p v-if="preparing" class="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 class="h-4 w-4 animate-spin" />{{ t(`${prefix}.preparing`) }}</p>
        <template v-if="prepared">
          <label class="block space-y-2 text-sm"><span>{{ t(`${prefix}.model`) }}</span><input v-model="model" list="group-probe-models" :disabled="busy" maxlength="200" class="h-10 w-full rounded-lg border border-border bg-background px-3 outline-none focus:ring-2 focus:ring-primary/30" :placeholder="t(`${prefix}.modelPlaceholder`)" @keydown.enter="probe"><datalist id="group-probe-models"><option v-for="option in models" :key="option.id" :value="option.id" /></datalist></label>
          <p v-if="unavailableModels" class="text-xs text-muted-foreground">{{ t(`${prefix}.modelListHint`) }}</p>
        </template>
        <p v-if="error" role="alert" class="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{{ readable(error) }}</p>
        <div v-if="result" role="status" class="rounded-lg p-3 text-sm" :class="result.result === 'ok' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-red-500/10 text-red-600 dark:text-red-400'">{{ readable(result.result) }} · {{ result.latencyMs == null ? '—' : `${(result.latencyMs / 1000).toFixed(2)}s` }} · {{ result.modelName }}</div>
        <footer class="flex justify-end gap-2">
          <button v-if="!prepared && !preparing" type="button" class="rounded-lg border border-border px-3 py-2 text-sm" @click="prepare">{{ t(`${prefix}.retry`) }}</button>
          <button v-if="prepared" type="button" :disabled="busy || !model.trim()" class="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" @click="probe"><Loader2 v-if="probing" class="h-4 w-4 animate-spin" /><Zap v-else class="h-4 w-4" />{{ t(`${prefix}.${probing ? 'probing' : 'start'}`) }}</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>
