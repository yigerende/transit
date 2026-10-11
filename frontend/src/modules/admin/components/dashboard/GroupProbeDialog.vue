<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useEventListener } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { Loader2, Save, X } from 'lucide-vue-next'
import { getGroupProbeConfig, prepareGroupProbe, saveGroupProbeConfig } from '../../api/connectionHealth'
import { connectionHealthMessageKey } from '../../composables/useConnectionHealth'
import type { AdminGroupHealth, GroupProbeMode, ManualProbeModelOption } from '../../types/connectionHealth'

const props = defineProps<{ group: AdminGroupHealth | null; open: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t, te } = useI18n()
const prefix = 'admin.connectionHealth.groupProbe'
const preparing = ref(false)
const saving = ref(false)
const loaded = ref(false)
const configured = ref(false)
const enabled = ref(true)
const key = ref('')
const hasCustomKey = ref(false)
const useAutoKey = ref(false)
const keySelection = () => ({ key: key.value.trim() || undefined, useAutoKey: useAutoKey.value && !key.value.trim() })
const intervalSeconds = ref<number | string>(60)
const models = ref<ManualProbeModelOption[]>([])
const model = ref('')
const probeMode = ref<GroupProbeMode>('real_model')
const probeModes: GroupProbeMode[] = ['real_model', 'arithmetic', 'first_token']
const unavailableModels = ref(false)
const error = ref('')
const validInterval = computed(() => Number.isInteger(Number(intervalSeconds.value)) && Number(intervalSeconds.value) >= 10 && Number(intervalSeconds.value) <= 86400)
const canSave = computed(() => loaded.value && !busy.value && model.value.trim() && validInterval.value)
let sequence = 0
const dialog = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
const restoreFocus = () => previousFocus?.focus()
onUnmounted(() => { sequence++; restoreFocus() })
const trapFocus = (event: KeyboardEvent) => {
  if (event.key !== 'Tab' || !dialog.value) return
  const elements = Array.from(dialog.value.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled)'))
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
const busy = computed(() => preparing.value || saving.value)
const readable = (key: string) => t(connectionHealthMessageKey(key, te))
const loadConfig = async () => {
  if (!props.group) return
  const group = props.group
  const current = ++sequence
  preparing.value = true
  error.value = ''
  try {
    const config = await getGroupProbeConfig(group.id)
    if (current !== sequence) return
    configured.value = Boolean(config)
    hasCustomKey.value = config?.hasCustomKey ?? false
    enabled.value = config?.enabled ?? true
    probeMode.value = config?.probeMode ?? 'real_model'
    intervalSeconds.value = config?.intervalSeconds ?? 60
    model.value = config?.model || group.recentProbes?.[0]?.modelName || ''
    loaded.value = true
  } catch (err) {
    if (current === sequence) error.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
  } finally {
    if (current === sequence) preparing.value = false
  }
}
const prepare = async () => {
  if (!props.group || busy.value) return
  const current = sequence
  preparing.value = true
  error.value = ''
  try {
    const response = await prepareGroupProbe(props.group.id, keySelection())
    if (current !== sequence) return
    models.value = response.models
    unavailableModels.value = response.modelListUnavailable || !response.models.length
    if (!model.value) model.value = response.models[0]?.id || ''
  } catch (err) {
    if (current === sequence) error.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
  } finally {
    if (current === sequence) preparing.value = false
  }
}
watch(() => [props.open, props.group?.id], async () => {
  sequence++
  preparing.value = false
  saving.value = false
  loaded.value = false
  if (!props.open || !props.group) { restoreFocus(); return }
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  configured.value = false
  key.value = ''
  hasCustomKey.value = false
  useAutoKey.value = false
  unavailableModels.value = false
  enabled.value = true
  intervalSeconds.value = 60
  probeMode.value = 'real_model'
  models.value = []
  model.value = ''
  void loadConfig()
  await nextTick()
  dialog.value?.focus()
})
const save = async () => {
  if (!props.group || !canSave.value) return
  const groupId = props.group.id
  const current = sequence
  saving.value = true
  error.value = ''
  try {
    await saveGroupProbeConfig(groupId, { model: model.value.trim(), probeMode: probeMode.value, intervalSeconds: Number(intervalSeconds.value), enabled: enabled.value, ...keySelection() })
    emit('saved')
    if (current === sequence) emit('close')
  } catch (err) {
    if (current === sequence) error.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
  } finally {
    if (current === sequence) saving.value = false
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
      <section ref="dialog" role="dialog" aria-modal="true" tabindex="-1" :aria-label="t(`${prefix}.title`)" class="max-h-[calc(100dvh-2rem)] w-full max-w-md space-y-5 overflow-y-auto rounded-xl border border-border bg-card p-5 text-card-foreground shadow-xl outline-none">
        <header class="flex items-center justify-between gap-3">
          <div class="min-w-0"><h2 class="font-semibold">{{ t(`${prefix}.title`) }}</h2><p class="mt-1 truncate text-sm text-muted-foreground">{{ group.name }} · {{ group.platform }}</p></div>
          <button type="button" :disabled="busy" :aria-label="t(`${prefix}.close`)" class="rounded p-1 text-muted-foreground hover:bg-surface disabled:opacity-40" @click="close"><X class="h-4 w-4" /></button>
        </header>
        <p class="text-sm leading-6 text-muted-foreground">{{ t(`${prefix}.hint`) }}</p>
        <p v-if="preparing" class="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 class="h-4 w-4 animate-spin" />{{ t(`${prefix}.preparing`) }}</p>
        <template v-if="loaded">
          <label class="block space-y-2 text-sm"><span>{{ t(`${prefix}.key`) }}</span><input v-model="key" type="password" autocomplete="new-password" :disabled="busy || !enabled" maxlength="4096" class="h-10 w-full rounded-lg border border-border bg-background px-3 outline-none focus:ring-2 focus:ring-primary/30" :placeholder="t(`${prefix}.${hasCustomKey && !useAutoKey ? 'keySavedPlaceholder' : 'keyPlaceholder'}`)" @input="models = []; unavailableModels = false"></label>
          <div v-if="hasCustomKey && !useAutoKey" class="flex items-center justify-between gap-2 text-xs text-muted-foreground"><span>{{ t(`${prefix}.keySaved`) }}</span><button type="button" :disabled="busy || !enabled" class="shrink-0 text-primary disabled:opacity-50" @click="useAutoKey = true; key = ''; models = []">{{ t(`${prefix}.useAutoKey`) }}</button></div>
          <label class="block space-y-2 text-sm"><span>{{ t(`${prefix}.model`) }}</span><input v-model="model" list="group-probe-models" :disabled="busy" maxlength="200" class="h-10 w-full rounded-lg border border-border bg-background px-3 outline-none focus:ring-2 focus:ring-primary/30" :placeholder="t(`${prefix}.modelPlaceholder`)" @keydown.enter="save"><datalist id="group-probe-models"><option v-for="option in models" :key="option.id" :value="option.id" /></datalist></label>
          <button type="button" :disabled="busy || !enabled" class="text-xs text-primary disabled:opacity-50" @click="prepare">{{ t(`${prefix}.fetchModels`) }}</button>
          <p v-if="unavailableModels" class="text-xs text-muted-foreground">{{ t(`${prefix}.modelListHint`) }}</p>
          <label class="block space-y-2 text-sm"><span>{{ t(`${prefix}.probeMode`) }}</span><select v-model="probeMode" :disabled="busy" class="h-10 w-full rounded-lg border border-border bg-background px-3 outline-none focus:ring-2 focus:ring-primary/30"><option v-for="mode in probeModes" :key="mode" :value="mode">{{ t(`${prefix}.probeModes.${mode}`) }}</option></select></label>
          <p class="text-xs leading-5 text-muted-foreground">{{ t(`${prefix}.probeModeHints.${probeMode}`) }}</p>
          <label class="block space-y-2 text-sm"><span>{{ t(`${prefix}.interval`) }}</span><input v-model="intervalSeconds" type="number" min="10" max="86400" step="1" :disabled="busy" class="h-10 w-full rounded-lg border border-border bg-background px-3 outline-none focus:ring-2 focus:ring-primary/30" @keydown.enter="save"></label>
          <p v-if="!validInterval" role="alert" class="text-xs text-destructive">{{ t(`${prefix}.intervalInvalid`) }}</p>
          <label class="flex items-center gap-2 text-sm"><input v-model="enabled" type="checkbox" :disabled="busy || !configured" class="h-4 w-4 accent-primary">{{ t(`${prefix}.enabled`) }}</label>
        </template>
        <p v-if="error" role="alert" class="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{{ readable(error) }}</p>
        <footer class="flex justify-end gap-2">
          <button v-if="!loaded && !preparing" type="button" class="rounded-lg border border-border px-3 py-2 text-sm" @click="loadConfig">{{ t(`${prefix}.retry`) }}</button>
          <button v-if="loaded" type="button" :disabled="!canSave" class="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" @click="save"><Loader2 v-if="saving" class="h-4 w-4 animate-spin" /><Save v-else class="h-4 w-4" />{{ t(`${prefix}.${saving ? 'saving' : 'save'}`) }}</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>
