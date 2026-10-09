<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import {
  Activity,
  Layers,
  Loader2,
  RefreshCw,
  Search,
  Settings2,
  Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { listUpstreamSites } from '../api/upstream'
import { connectionHealthMessageKey, useConnectionHealth } from '../composables/useConnectionHealth'
import ChannelHealthCard from '../components/dashboard/ChannelHealthCard.vue'
import ProbeHistoryStrip from '../components/dashboard/ProbeHistoryStrip.vue'
import GroupProbeDialog from '../components/dashboard/GroupProbeDialog.vue'
import ConnectionHealthEventsDialog from '../components/dashboard/ConnectionHealthEventsDialog.vue'
import GroupHealthSetupDrawer from '../components/dashboard/GroupHealthSetupDrawer.vue'
import ManualOneTimeProbeDialog from '../components/dashboard/ManualOneTimeProbeDialog.vue'
import type { ManualProbeTargetSummary } from '../components/dashboard/ManualOneTimeProbeDialog.vue'
import PolicyConfigDrawer from '../components/dashboard/PolicyConfigDrawer.vue'
import type { OwnGroupOption } from '../components/dashboard/PolicyConfigDrawer.vue'
import ProbePolicyListDialog from '../components/dashboard/ProbePolicyListDialog.vue'
import type {
  AdminGroupAccount,
  AdminGroupHealth,
  ConnectionHealthPolicy,
  PolicyInput,
} from '../types/connectionHealth'
import { resolveConnectionHealthStrategyMode } from '../utils/connectionHealthPolicy'

const { t, te } = useI18n()
const {
  groups,
  adminGroups,
  events,
  policies,
  isLoading,
  errorKey,
  loadAll,
  loadEvents,
  loadPolicies,
  removePolicy,
  savePolicy,
} = useConnectionHealth()

const searchText = ref('')
const selectedType = ref('')
const selectedGroupId = ref('')
const groupProbeOpen = ref(false)
const probeGroup = ref<AdminGroupHealth | null>(null)
const selectedConnectionId = ref('')
const eventsDialogOpen = ref(false)
const siteNameMap = ref<Map<string, string>>(new Map())

const groupTypes = ['public', 'exclusive', 'subscription']
const groupTypeLabel = (type: string): string => t(`admin.connectionHealth.groupTypes.${groupTypes.includes(type) ? type : 'public'}`)

const filteredGroups = computed(() => {
  const keyword = searchText.value.trim().toLocaleLowerCase()
  return adminGroups.value.filter((group) => {
    if (selectedType.value && group.type !== selectedType.value) return false
    if (!keyword) return true
    return group.name.toLocaleLowerCase().includes(keyword)
      || group.platform.toLocaleLowerCase().includes(keyword)
      || group.accounts.some((account) => (account.name || account.id).toLocaleLowerCase().includes(keyword))
  })
})

const readableMessage = (rawKey: string): string => t(connectionHealthMessageKey(rawKey, te))

const selectedGroup = computed(() => filteredGroups.value.find(group => group.id === selectedGroupId.value) ?? filteredGroups.value[0] ?? null)
watch(filteredGroups, (next) => {
  if (!next.some(group => group.id === selectedGroupId.value)) selectedGroupId.value = next[0]?.id ?? ''
}, { immediate: true })
const openGroupProbe = (group: AdminGroupHealth) => {
  probeGroup.value = group
  groupProbeOpen.value = true
}
const groupProbeState = (group: AdminGroupHealth): string => {
  if (group.probeHistoryError) return 'loadError'
  const latest = group.recentProbes?.[0]
  return latest ? (latest.result === 'ok' ? 'healthy' : 'unhealthy') : 'pending'
}
const refreshProbeResults = async () => { await loadAll({ silent: true }) }

const loadSiteNames = async () => {
  try {
    const sites = await listUpstreamSites()
    siteNameMap.value = new Map(sites.map((site) => [site.id, site.name]))
  } catch {
    // 站点名称仅用于事件展示，失败时保留 ID，不阻塞健康主流程。
  }
}

onMounted(() => {
  void loadAll()
  void loadEvents()
  void loadPolicies()
  void loadSiteNames()
})

const documentVisibility = useDocumentVisibility()
let autoRefreshInFlight = false
const autoRefresh = async () => {
  if (documentVisibility.value !== 'visible' || autoRefreshInFlight) return
  autoRefreshInFlight = true
  try {
    await Promise.all([
      loadAll({ silent: true }),
      ...(eventsDialogOpen.value ? [loadEvents(selectedConnectionId.value || undefined)] : []),
    ])
  } finally {
    autoRefreshInFlight = false
  }
}
// immediate=false 会让 VueUse 的 interval 保持暂停；这里只关闭首次回调，计时器本身必须启动。
useIntervalFn(() => void autoRefresh(), 30_000, { immediate: true, immediateCallback: false })
watch(documentVisibility, (visibility) => {
  if (visibility === 'visible') void autoRefresh()
})

const refresh = async () => {
  await Promise.all([loadAll(), loadPolicies(), loadEvents(selectedConnectionId.value || undefined)])
}

const siteName = (siteId: string): string => siteNameMap.value.get(siteId) ?? siteId

// 分组启用/管理抽屉。
const setupDrawerOpen = ref(false)
const setupGroup = ref<AdminGroupHealth | null>(null)

const openSetup = (group: AdminGroupHealth) => {
  setupGroup.value = group
  setupDrawerOpen.value = true
}

const onSetupSaved = async () => {
  setupDrawerOpen.value = false
  await Promise.all([loadAll({ silent: true }), loadPolicies()])
}

// 手动探活记录历史，不改变策略状态或触发远端动作。
const probeDialogOpen = ref(false)
const probeDialogTarget = ref<ManualProbeTargetSummary | null>(null)

const onProbeAccount = (group: AdminGroupHealth, account: AdminGroupAccount) => {
  if (!account.probeAvailable) return
  probeDialogTarget.value = {
    targetId: account.targetId,
    accountName: account.name || account.id,
    platform: group.platform,
    type: account.type,
    status: account.status,
    groupName: group.name,
  }
  probeDialogOpen.value = true
}

// 策略探活事件。
const openAllEvents = async () => {
  selectedConnectionId.value = ''
  await loadEvents()
  eventsDialogOpen.value = true
}

const onViewEventsAccount = async (account: AdminGroupAccount) => {
  selectedConnectionId.value = account.targetId
  await loadEvents(account.targetId)
  eventsDialogOpen.value = true
}

const showAllEvents = async () => {
  selectedConnectionId.value = ''
  await loadEvents()
}

// 高级策略列表/编辑继续保留，但退出首次主流程。
const policyListDialogOpen = ref(false)
const policyDrawerOpen = ref(false)
const editingPolicy = ref<ConnectionHealthPolicy | null>(null)
const deletingPolicyId = ref('')
const deletePolicyError = ref('')
const ownGroupOptions = computed<OwnGroupOption[]>(() => groups.value.map((group) => ({ id: group.ownGroupId, name: group.ownGroupName || group.ownGroupId })))

const openCreatePolicy = () => {
  editingPolicy.value = null
  policyDrawerOpen.value = true
}

const openEditPolicy = (policy: ConnectionHealthPolicy) => {
  editingPolicy.value = policy
  policyDrawerOpen.value = true
}

const handleSavePolicy = async (input: PolicyInput) => {
  if (await savePolicy(input)) {
    policyDrawerOpen.value = false
    await loadAll({ silent: true })
  }
}

const togglePolicyEnabled = async (policy: ConnectionHealthPolicy) => {
  await savePolicy({
    id: policy.id,
    name: policy.name,
    enabled: !policy.enabled,
    ownGroupId: policy.ownGroupId,
    ownGroupName: policy.ownGroupName,
    probeIntervalSeconds: policy.probeIntervalSeconds,
    failureThreshold: policy.failureThreshold,
    successThreshold: policy.successThreshold,
    cooldownSeconds: policy.cooldownSeconds,
    observationSeconds: policy.observationSeconds,
    recoveryStepPercent: policy.recoveryStepPercent,
    dailyProbeBudget: policy.dailyProbeBudget,
    autoDegradeEnabled: policy.autoDegradeEnabled,
    autoRemoteActionEnabled: policy.autoRemoteActionEnabled,
    priorityMode: policy.priorityMode ?? 'none',
    strategyMode: resolveConnectionHealthStrategyMode(policy),
    modelTargets: policy.modelTargets.map((model) => ({
      id: model.id,
      modelName: model.modelName,
      providerFamily: model.providerFamily,
      enabled: model.enabled,
      probePrompt: model.probePrompt,
      maxProbeTokens: model.maxProbeTokens,
    })),
  })
  await loadAll({ silent: true })
}

const handleDeletePolicy = async (policy: ConnectionHealthPolicy) => {
  if (deletingPolicyId.value) return
  deletingPolicyId.value = policy.id
  deletePolicyError.value = ''
  try {
    if (await removePolicy(policy.id)) {
      await loadAll({ silent: true })
    } else {
      deletePolicyError.value = readableMessage(errorKey.value)
    }
  } finally {
    deletingPolicyId.value = ''
  }
}
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold text-foreground">{{ t('admin.connectionHealth.title') }}</h1>
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="secondary" size="sm" @click="policyListDialogOpen = true">
          <Settings2 class="h-4 w-4" />{{ t('admin.connectionHealth.topActions.policies') }}
        </Button>
        <Button variant="secondary" size="sm" @click="openAllEvents">
          <Activity class="h-4 w-4" />{{ t('admin.connectionHealth.topActions.events') }}
        </Button>
        <Button variant="secondary" size="sm" :disabled="isLoading" @click="refresh">
          <Loader2 v-if="isLoading" class="h-4 w-4 animate-spin" />
          <RefreshCw v-else class="h-4 w-4" />{{ t('admin.connectionHealth.refresh') }}
        </Button>
      </div>
    </header>

    <p v-if="errorKey" role="alert" class="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">{{ readableMessage(errorKey) }}</p>

    <section class="grid min-h-[34rem] min-w-0 rounded-xl border border-border/70 bg-card lg:grid-cols-[18rem_minmax(0,1fr)]">
      <aside class="min-w-0 border-b border-border/60 lg:border-b-0 lg:border-r">
        <div class="space-y-2 border-b border-border/60 p-4">
          <div class="relative">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input v-model="searchText" type="search" :aria-label="t('admin.connectionHealth.filters.searchGroup')" :placeholder="t('admin.connectionHealth.filters.searchGroup')" class="h-9 w-full rounded-lg border border-border/70 bg-background pl-9 pr-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-primary/30">
          </div>
          <select v-model="selectedType" :aria-label="t('admin.connectionHealth.filters.allTypes')" class="h-9 w-full rounded-lg border border-border/70 bg-background px-3 text-sm text-foreground">
            <option value="">{{ t('admin.connectionHealth.filters.allTypes') }}</option>
            <option v-for="type in groupTypes" :key="type" :value="type">{{ groupTypeLabel(type) }}</option>
          </select>
        </div>
        <nav class="max-h-80 space-y-1.5 overflow-y-auto p-2 lg:max-h-[calc(100dvh-17rem)]" :aria-label="t('admin.connectionHealth.groupListLabel')">
          <div v-if="isLoading && !adminGroups.length" class="space-y-2 p-2" aria-busy="true"><div v-for="i in 5" :key="i" class="h-24 animate-pulse rounded-lg bg-surface" /></div>
          <div v-for="group in filteredGroups" :key="group.id" class="rounded-lg border p-3 transition-colors" :class="selectedGroup?.id === group.id ? 'border-primary/20 bg-primary/[0.06]' : 'border-transparent hover:bg-surface/60'">
            <div class="flex items-start gap-2">
              <button type="button" class="min-w-0 flex-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" :aria-current="selectedGroup?.id === group.id ? 'true' : undefined" @click="selectedGroupId = group.id">
                <span class="block truncate text-sm font-medium text-foreground" :title="group.name">{{ group.name }}</span>
                <span class="mt-1 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                  <span :class="groupProbeState(group) === 'healthy' ? 'text-emerald-600 dark:text-emerald-400' : groupProbeState(group) === 'unhealthy' ? 'text-red-600 dark:text-red-400' : ''">{{ t('admin.connectionHealth.cards.status.' + groupProbeState(group)) }}</span>
                  <span>{{ group.multiplierDisplay || '—' }}</span>
                </span>
              </button>
              <button type="button" :disabled="group.groupProbeSupported === false" class="rounded-md p-1.5 text-muted-foreground hover:bg-primary/10 hover:text-primary disabled:opacity-30" :aria-label="t('admin.connectionHealth.groupProbe.action', { name: group.name })" :title="t(group.groupProbeSupported === false ? 'admin.connectionHealth.groupProbe.unsupported' : 'admin.connectionHealth.groupProbe.buttonHint')" @click="openGroupProbe(group)"><Zap class="h-4 w-4" /></button>
            </div>
            <ProbeHistoryStrip class="mt-2.5" :samples="group.recentProbes" :limit="20" compact :unavailable="Boolean(group.probeHistoryError)" />
          </div>
          <p v-if="!isLoading && !filteredGroups.length" class="px-3 py-8 text-center text-sm text-muted-foreground">{{ t(adminGroups.length ? 'admin.connectionHealth.cards.noMatches' : 'admin.connectionHealth.adminEmpty') }}</p>
        </nav>
      </aside>

      <div v-if="selectedGroup" class="min-w-0">
        <header class="flex flex-wrap items-start justify-between gap-3 border-b border-border/60 p-5">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2"><h2 class="break-words text-lg font-semibold text-foreground">{{ selectedGroup.name }}</h2><span class="rounded bg-surface px-2 py-0.5 text-xs text-muted-foreground">{{ selectedGroup.platform }}</span></div>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.connectionHealth.groupProbe.channelCount', { count: selectedGroup.monitoredAccountCount ?? 0, total: selectedGroup.accountCount }) }}</p>
          </div>
          <Button variant="secondary" size="sm" @click="openSetup(selectedGroup)"><Settings2 class="h-4 w-4" />{{ t('admin.connectionHealth.groupDetail.manageMonitoring') }}</Button>
        </header>
        <div class="space-y-3 p-3 sm:p-5">
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>{{ t('admin.connectionHealth.groupProbe.channels') }}</span>
            <div class="flex flex-wrap gap-3"><span class="inline-flex items-center gap-1"><i class="h-2 w-2 rounded-sm bg-emerald-500" />{{ t('admin.connectionHealth.cards.success') }}</span><span class="inline-flex items-center gap-1"><i class="h-2 w-2 rounded-sm bg-amber-400" />{{ t('admin.connectionHealth.cards.slowLegend') }}</span><span class="inline-flex items-center gap-1"><i class="h-2 w-2 rounded-sm bg-red-500" />{{ t('admin.connectionHealth.cards.failure') }}</span></div>
          </div>
          <p v-if="selectedGroup.accountsError" role="alert" class="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{{ readableMessage(selectedGroup.accountsError) }}</p>
          <p v-else-if="!selectedGroup.accounts.length" class="py-16 text-center text-sm text-muted-foreground">{{ t('admin.connectionHealth.groupDetail.empty') }}</p>
          <p v-if="(selectedGroup.priorityConflictCount ?? 0) > 0" class="text-xs text-amber-600">{{ t('admin.connectionHealth.cards.priorityConflict', { count: selectedGroup.priorityConflictCount }) }}</p>
          <ChannelHealthCard v-for="account in selectedGroup.accounts" :key="account.targetId" :account="account" :history-unavailable="Boolean(selectedGroup.probeHistoryError)" @probe="onProbeAccount(selectedGroup, $event)" @view-events="onViewEventsAccount" />
        </div>
      </div>
      <div v-else class="flex min-h-64 flex-col items-center justify-center gap-3 p-6 text-center text-muted-foreground"><Layers class="h-8 w-8 opacity-40" /><p class="text-sm">{{ t('admin.connectionHealth.groupProbe.selectGroup') }}</p></div>
    </section>

    <GroupProbeDialog :open="groupProbeOpen" :group="probeGroup" @close="groupProbeOpen = false" @probed="refreshProbeResults" />

    <GroupHealthSetupDrawer
      :open="setupDrawerOpen"
      :group="setupGroup"
      :policies="policies"
      @close="setupDrawerOpen = false"
      @saved="onSetupSaved"
    />

    <ManualOneTimeProbeDialog
      :open="probeDialogOpen"
      :target="probeDialogTarget"
      @probed="refreshProbeResults"
      @close="probeDialogOpen = false"
    />

    <ProbePolicyListDialog
      :open="policyListDialogOpen"
      :policies="policies"
      :deleting-policy-id="deletingPolicyId"
      :delete-error="deletePolicyError"
      @close="policyListDialogOpen = false"
      @create="openCreatePolicy"
      @delete="handleDeletePolicy"
      @edit="openEditPolicy"
      @toggle="togglePolicyEnabled"
    />

    <PolicyConfigDrawer
      :open="policyDrawerOpen"
      :policy="editingPolicy"
      :own-group-options="ownGroupOptions"
      @close="policyDrawerOpen = false"
      @save="handleSavePolicy"
    />

    <ConnectionHealthEventsDialog
      :open="eventsDialogOpen"
      :events="events"
      :groups="groups"
      :admin-groups="adminGroups"
      :policies="policies"
      :selected-connection-id="selectedConnectionId"
      :site-name="siteName"
      @close="eventsDialogOpen = false"
      @view-all="showAllEvents"
    />
  </div>
</template>
