<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Trash2 } from 'lucide-vue-next'
import type { LatencyPriorityConfig, ModelTargetInput } from '../../types/connectionHealth'
import { defaultLatencyWeights } from '../../utils/latencyPriority'

const props = defineProps<{ modelValue: LatencyPriorityConfig; models: ModelTargetInput[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: LatencyPriorityConfig] }>()
const { t } = useI18n()
const prefix = 'admin.connectionHealth.latencyPriority'
const enabledModels = computed(() => props.models.filter(m => m.enabled && m.modelName.trim()))
const patch = (value: Partial<LatencyPriorityConfig>) => emit('update:modelValue', { ...props.modelValue, ...value })
const setCount = (value: number) => {
  const size = Math.max(1, Math.min(20, Math.trunc(value) || 1))
  const weights = defaultLatencyWeights(size)
  patch({ sampleCount: size, minSamples: Math.min(size, props.modelValue.minSamples), weights })
}
const setWeight = (index: number, value: number) => {
  const weights = [...props.modelValue.weights]; weights[index] = value; patch({ weights })
}
const setBand = (index: number, key: 'minSeconds' | 'maxSeconds' | 'priority', value: number | null) => {
  const bands = props.modelValue.bands.map(b => ({ ...b }))
  if (key === 'maxSeconds') bands[index].maxSeconds = value
  else bands[index][key] = value ?? 0
  patch({ bands })
}
const addBand = () => {
  const bands = props.modelValue.bands.map(b => ({ ...b }))
  const last = bands.at(-1)!
  const start = last.maxSeconds ?? last.minSeconds + 5
  last.maxSeconds = start
  bands.push({ minSeconds: start, maxSeconds: null, priority: last.priority + 1 })
  patch({ bands })
}
const removeBand = (index: number) => patch({ bands: props.modelValue.bands.filter((_, i) => i !== index) })
const numberValue = (event: Event) => Number((event.target as HTMLInputElement).value)
</script>

<template>
  <div class="space-y-4 rounded-lg border border-border/50 bg-surface/20 p-3">
    <p class="text-xs leading-5 text-muted-foreground">{{ t(`${prefix}.help`) }}</p>
    <label class="latency-field">
      {{ t(`${prefix}.model`) }}
      <select class="latency-input" :value="modelValue.modelName" @change="patch({ modelName: ($event.target as HTMLSelectElement).value })">
        <option value="">{{ t(`${prefix}.firstModel`) }}</option>
        <option v-for="model in enabledModels" :key="model.modelName" :value="model.modelName">{{ model.modelName }}</option>
      </select>
    </label>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <label class="latency-field">{{ t(`${prefix}.sampleCount`) }}<input class="latency-input" type="number" min="1" max="20" :value="modelValue.sampleCount" @input="setCount(numberValue($event))" /></label>
      <label class="latency-field">{{ t(`${prefix}.maxAge`) }}<input class="latency-input" type="number" min="1" max="86400" :value="modelValue.maxAgeSeconds" @input="patch({ maxAgeSeconds: numberValue($event) })" /></label>
      <label class="latency-field">{{ t(`${prefix}.minSamples`) }}<input class="latency-input" type="number" min="1" :max="modelValue.sampleCount" :value="modelValue.minSamples" @input="patch({ minSamples: numberValue($event) })" /></label>
      <label class="latency-field">{{ t(`${prefix}.hysteresis`) }}<input class="latency-input" type="number" min="0" step="0.1" :value="modelValue.hysteresisSeconds" @input="patch({ hysteresisSeconds: numberValue($event) })" /></label>
    </div>
    <p class="text-xs leading-5 text-muted-foreground">{{ t(`${prefix}.windowHelp`) }}</p>
    <div class="space-y-2">
      <p class="text-xs font-medium">{{ t(`${prefix}.weights`) }}</p>
      <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
        <label v-for="(weight, i) in modelValue.weights" :key="i" class="latency-field">{{ t(`${prefix}.weightItem`, { n: i + 1 }) }}<input class="latency-input" type="number" min="0.01" step="any" :value="weight" @input="setWeight(i, numberValue($event))" /></label>
      </div>
    </div>
    <p class="text-xs leading-5 text-muted-foreground">{{ t(`${prefix}.bandsHelp`) }}</p>
    <div v-for="(band, i) in modelValue.bands" :key="i" class="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2">
      <label class="latency-field">{{ t(`${prefix}.from`) }}<input class="latency-input" type="number" min="0" step="0.1" :value="band.minSeconds" @input="setBand(i, 'minSeconds', numberValue($event))" /></label>
      <label class="latency-field">{{ t(`${prefix}.to`) }}<input class="latency-input" type="number" min="0" step="0.1" :placeholder="t(`${prefix}.unbounded`)" :value="band.maxSeconds ?? ''" @input="setBand(i, 'maxSeconds', ($event.target as HTMLInputElement).value === '' ? null : numberValue($event))" /></label>
      <label class="latency-field">{{ t(`${prefix}.priority`) }}<input class="latency-input" type="number" min="0" :value="band.priority" @input="setBand(i, 'priority', numberValue($event))" /></label>
      <button type="button" class="mb-1 rounded p-2 text-muted-foreground hover:text-red-500 disabled:opacity-40" :aria-label="t(`${prefix}.removeBand`, { n: i + 1 })" :disabled="modelValue.bands.length <= 1" @click="removeBand(i)"><Trash2 class="h-4 w-4" /></button>
    </div>
    <button type="button" class="inline-flex items-center gap-1 text-xs text-primary disabled:opacity-40" :disabled="modelValue.bands.length >= 20" @click="addBand"><Plus class="h-4 w-4" />{{ t(`${prefix}.addBand`) }}</button>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <label class="latency-field">{{ t(`${prefix}.insufficientPriority`) }}<input class="latency-input" type="number" min="0" :value="modelValue.insufficientPriority" @input="patch({ insufficientPriority: numberValue($event) })" /></label>
      <label class="latency-field">{{ t(`${prefix}.degradedPriority`) }}<input class="latency-input" type="number" min="0" :value="modelValue.degradedPriority" @input="patch({ degradedPriority: numberValue($event) })" /></label>
      <label class="latency-field">{{ t(`${prefix}.suspendedPriority`) }}<input class="latency-input" type="number" min="0" :value="modelValue.suspendedPriority" @input="patch({ suspendedPriority: numberValue($event) })" /></label>
    </div>
    <p class="text-xs leading-5 text-muted-foreground">{{ t(`${prefix}.sharedHelp`) }}</p>
  </div>
</template>

<style scoped>
.latency-field { @apply flex min-w-0 flex-col gap-1.5 text-xs font-medium; }
.latency-input { @apply h-9 w-full min-w-0 rounded-lg border border-border/60 bg-background px-2 text-sm font-normal text-foreground; }
</style>
