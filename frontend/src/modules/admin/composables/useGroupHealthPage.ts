import { onScopeDispose, ref } from 'vue'
import { getConnectionHealthGroupDetail, getConnectionHealthGroupSummaries } from '../api/connectionHealth'
import type { AdminGroupHealth } from '../types/connectionHealth'

// Page-local data cannot survive logout/workspace navigation into another user.
// Existing cards stay visible while fresh state is read; histories are only
// requested for the selected group (or a group whose editor is explicitly opened).
export function useGroupHealthPage(selectedGroupId: () => string) {
  const adminGroups = ref<AdminGroupHealth[]>([])
  const isLoading = ref(false)
  const errorKey = ref('')
  const detailErrors = ref<Record<string, string>>({})
  const detailLoading = ref(new Set<string>())
  const detailsLoaded = ref(new Set<string>())
  const pendingDetails = new Map<string, Promise<AdminGroupHealth | null>>()
  let pendingList: Promise<void> | null = null
  let visibleLoads = 0
  let disposed = false
  onScopeDispose(() => { disposed = true })

  async function loadGroupDetail(id: string, force = false): Promise<AdminGroupHealth | null> {
    if (!id || disposed) return null
    const pending = pendingDetails.get(id)
    if (pending) {
      if (!force) return pending
      await pending
      if (disposed) return null
      return loadGroupDetail(id)
    }
    detailLoading.value.add(id)
    delete detailErrors.value[id]
    const request = (async () => {
      try {
        const group = await getConnectionHealthGroupDetail(id)
        if (disposed) return null
        if (group.accountsError || group.accountsLoaded === false) {
          throw new Error(group.accountsError || 'admin.connectionHealth.errors.accountsFetch')
        }
        adminGroups.value = adminGroups.value.map(old => old.id === id ? group : old)
        detailsLoaded.value.add(id)
        return group
      } catch (err) {
        if (!disposed) detailErrors.value[id] = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
        return null
      } finally {
        pendingDetails.delete(id)
        if (!disposed) detailLoading.value.delete(id)
      }
    })()
    pendingDetails.set(id, request)
    return request
  }

  async function loadAll(options: { silent?: boolean; force?: boolean } = {}): Promise<void> {
    if (disposed) return
    if (!options.silent) { visibleLoads++; isLoading.value = true }
    try {
      await loadList(options)
    } finally {
      if (!options.silent) {
        visibleLoads--
        if (!disposed) isLoading.value = visibleLoads > 0
      }
    }
  }

  async function loadList(options: { silent?: boolean; force?: boolean }): Promise<void> {
    if (disposed) return
    if (pendingList) {
      if (!options.force) return pendingList
      await pendingList
      if (disposed) return
      return loadList({ ...options, force: false })
    }
    errorKey.value = ''
    const request = (async () => {
      try {
        const summaries = await getConnectionHealthGroupSummaries()
        if (disposed) return
        applySummaries(summaries)
        const ids = new Set(summaries.map(group => group.id))
        detailsLoaded.value = new Set([...detailsLoaded.value].filter(id => ids.has(id)))
        const selected = selectedGroupId()
        const id = ids.has(selected) ? selected : summaries[0]?.id
        if (id) await loadGroupDetail(id, options.force)
        // The first sidebar response can precede the account directory. Once
        // details are ready, fill counts and legacy policy bindings for all rows.
        if (!disposed && summaries.some(group => group.accountsLoaded === false)) {
          const complete = await getConnectionHealthGroupSummaries()
          if (!disposed) applySummaries(complete)
        }
      } catch (err) {
        if (!disposed) errorKey.value = err instanceof Error ? err.message : 'admin.connectionHealth.errors.request'
      } finally {
        pendingList = null
      }
    })()
    pendingList = request
    return request
  }

  function applySummaries(summaries: AdminGroupHealth[]) {
    const previous = new Map(adminGroups.value.map(group => [group.id, group]))
    // Summary responses contain no channel histories. Preserve rendered cards
    // until the separate detail request supplies their next complete state.
    adminGroups.value = summaries.map(summary => {
      const old = previous.get(summary.id)
      return old && detailsLoaded.value.has(summary.id)
        ? { ...summary, accounts: old.accounts, accountsLoaded: old.accountsLoaded }
        : summary
    })
  }

  return { adminGroups, isLoading, errorKey, detailErrors, detailLoading, detailsLoaded, loadAll, loadGroupDetail }
}
