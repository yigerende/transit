import type { AdminGroupAccount, GroupProbeSample } from '../types/connectionHealth'

export const channelAutomationEnabled = (account: AdminGroupAccount): boolean => {
  if (account.excludedFromGroupPolicy || account.qualitySelected === false) return false
  return account.hasEnabledPolicy
    ?? account.assignedPolicies?.some(policy => policy.enabled)
    ?? account.hasEnabledProbePolicy
    ?? false
}

export const latestChannelProbe = (account: AdminGroupAccount): GroupProbeSample | undefined =>
  account.recentProbes?.reduce<GroupProbeSample | undefined>((latest, sample) => {
    if (!latest) return sample
    const difference = Date.parse(sample.createdAt) - Date.parse(latest.createdAt)
    return difference > 0 || (difference === 0 && sample.id > latest.id) ? sample : latest
  }, undefined)

export const channelsByLatestLatency = (accounts: AdminGroupAccount[]): AdminGroupAccount[] =>
  accounts.map(account => {
    const latest = latestChannelProbe(account)
    // A fast error is not a fast conversation. Only the latest successful
    // result can put a channel ahead of failed or unprobed channels.
    const latency = latest?.result === 'ok' ? latest.latencyMs : null
    return { account, succeeded: latest?.result === 'ok', latency: latency != null && Number.isFinite(latency) && latency >= 0 ? latency : Infinity }
  }).sort((a, b) => Number(b.succeeded) - Number(a.succeeded) || a.latency - b.latency).map(({ account }) => account)
