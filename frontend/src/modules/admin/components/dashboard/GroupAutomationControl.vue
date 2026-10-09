<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDownUp, Loader2, Pencil, Plus, Radar, ShieldCheck } from 'lucide-vue-next'
import type { ConnectionHealthPolicy } from '../../types/connectionHealth'
import { automationCapability } from '../../utils/connectionHealthPolicy'

const props = defineProps<{
  groupName: string
  policies: ConnectionHealthPolicy[]
  usageCounts: Map<string, number>
  busyPolicyId: string
  unavailable?: boolean
}>()
const emit = defineEmits<{
  edit: [policy: ConnectionHealthPolicy]
  toggle: [policy: ConnectionHealthPolicy]
  setup: []
}>()
const { t } = useI18n()
const p = 'admin.connectionHealth.groupAutomation'
const expanded = ref(false)
const single = computed(() => props.policies.length === 1 ? props.policies[0] : null)
const active = computed(() => props.policies.filter(policy => policy.enabled))
const capability = computed(() => {
  const modes = new Set((active.value.length ? active.value : props.policies).map(automationCapability))
  return modes.size === 1 ? [...modes][0]! : 'mixed'
})
const icon = computed(() => capability.value === 'suspend' || capability.value === 'localSuspend'
  ? ShieldCheck : capability.value === 'priority' || capability.value === 'priorityOnly' ? ArrowDownUp : Radar)
const tone = computed(() => !active.value.length ? 'text-muted-foreground'
  : capability.value === 'suspend' || capability.value === 'localSuspend' ? 'text-amber-600 dark:text-amber-400'
    : capability.value === 'priority' || capability.value === 'priorityOnly' ? 'text-primary' : 'text-muted-foreground')
const sharedHint = (policy: ConnectionHealthPolicy) => (props.usageCounts.get(policy.id) ?? 0) > 1
  ? t(`${p}.shared`, { count: props.usageCounts.get(policy.id) }) : ''
const policyHint = (policy: ConnectionHealthPolicy) => [policy.name,
  t(`${p}.${policy.enabled ? 'enabled' : 'disabled'}`),
  t(`${p}.hints.${automationCapability(policy)}`), sharedHint(policy),
].filter(Boolean).join(' · ')
const toggleLabel = (policy: ConnectionHealthPolicy) => t(`${p}.${policy.enabled ? 'disable' : 'enable'}`, { group: props.groupName, policy: policy.name })
</script>

<template>
  <div class="mt-2 text-[11px]" :aria-label="t(`${p}.groupLabel`, { group: groupName })">
    <div v-if="unavailable" class="flex h-6 items-center gap-1.5 text-muted-foreground" :title="t(`${p}.unavailableHint`)">
      <Radar class="h-3.5 w-3.5" />{{ t(`${p}.unavailable`) }}
    </div>
    <div v-else-if="!policies.length" class="flex min-h-6 items-center justify-between gap-2 text-muted-foreground">
      <span class="inline-flex items-center gap-1.5"><Radar class="h-3.5 w-3.5" />{{ t(`${p}.none`) }}</span>
      <button type="button" class="rounded p-1.5 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :aria-label="t(`${p}.configure`, { group: groupName })" :title="t(`${p}.configure`, { group: groupName })" @click.stop="emit('setup')"><Plus class="h-3.5 w-3.5" /></button>
    </div>
    <div v-else class="flex min-h-6 items-center gap-1">
      <div class="flex min-w-0 flex-1 items-center gap-1.5 py-1" :class="tone" :title="single ? policyHint(single) : t(`${p}.multipleHint`, { count: policies.length, enabled: active.length })">
        <component :is="icon" class="h-3.5 w-3.5 shrink-0" />
        <span class="truncate">{{ t(`${p}.modes.${capability}`) }}</span>
        <span v-if="policies.length > 1" class="shrink-0 text-[10px] opacity-65">{{ policies.length }}</span>
      </div>
      <template v-if="single">
        <button type="button" class="rounded p-1.5 text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" :disabled="Boolean(busyPolicyId)" :aria-label="t(`${p}.edit`, { group: groupName })" :title="t(`${p}.edit`, { group: groupName })" @click.stop="emit('edit', single)"><Pencil class="h-3 w-3" /></button>
        <button type="button" role="switch" :aria-checked="single.enabled" :aria-label="toggleLabel(single)" :title="[toggleLabel(single), sharedHint(single)].filter(Boolean).join(' · ')" :disabled="Boolean(busyPolicyId)" class="flex h-6 shrink-0 items-center rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" @click.stop="emit('toggle', single)">
          <Loader2 v-if="busyPolicyId === single.id" class="mx-1.5 h-4 w-4 animate-spin text-muted-foreground" />
          <span v-else class="relative h-4 w-7 rounded-full transition-colors" :class="single.enabled ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="single.enabled ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
        </button>
      </template>
      <button v-else type="button" class="rounded p-1.5 text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :aria-label="t(`${p}.manage`, { group: groupName, count: policies.length })" :title="t(`${p}.manage`, { group: groupName, count: policies.length })" :aria-expanded="expanded" @click.stop="expanded = !expanded"><Pencil class="h-3 w-3" /></button>
    </div>
    <div v-if="policies.length > 1 && expanded" class="mt-1 space-y-1 border-l border-border/60 pl-2">
      <div v-for="policy in policies" :key="policy.id" class="flex items-center gap-1">
        <span class="min-w-0 flex-1 truncate py-1 text-muted-foreground" :title="policyHint(policy)">{{ policy.name }}</span>
        <button type="button" class="rounded p-1.5 text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" :disabled="Boolean(busyPolicyId)" :aria-label="t(`${p}.editPolicy`, { policy: policy.name })" :title="t(`${p}.editPolicy`, { policy: policy.name })" @click.stop="emit('edit', policy)"><Pencil class="h-3 w-3" /></button>
        <button type="button" role="switch" :aria-checked="policy.enabled" :aria-label="toggleLabel(policy)" :title="[toggleLabel(policy), sharedHint(policy)].filter(Boolean).join(' · ')" :disabled="Boolean(busyPolicyId)" class="flex h-6 shrink-0 items-center rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-40" @click.stop="emit('toggle', policy)">
          <Loader2 v-if="busyPolicyId === policy.id" class="mx-1.5 h-4 w-4 animate-spin text-muted-foreground" />
          <span v-else class="relative h-4 w-7 rounded-full transition-colors" :class="policy.enabled ? 'bg-primary' : 'bg-muted-foreground/25'" aria-hidden="true"><span class="absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white transition-transform" :class="policy.enabled ? 'translate-x-3.5' : 'translate-x-0.5'" /></span>
        </button>
      </div>
      <button type="button" class="py-1 text-muted-foreground hover:text-primary" @click="emit('setup')">{{ t(`${p}.assignments`) }}</button>
    </div>
  </div>
</template>
