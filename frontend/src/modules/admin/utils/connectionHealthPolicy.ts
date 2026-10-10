import type { AdminGroupHealth, ConnectionHealthPolicy, ConnectionHealthStrategyMode, PolicyInput } from '../types/connectionHealth'

// 兼容尚未返回 strategyMode 的旧后端：新版前端通过旧后端保存的仅倍率策略会表现为
// multiplier + 无模型目标 + 关闭自动降级，按该稳定特征恢复其真实模式。
export const resolveConnectionHealthStrategyMode = (
  policy: Pick<ConnectionHealthPolicy, 'strategyMode' | 'priorityMode' | 'autoDegradeEnabled' | 'modelTargets'>,
): ConnectionHealthStrategyMode => {
  if (policy.strategyMode === 'multiplier_only') return 'multiplier_only'
  if (
    !policy.strategyMode
    && policy.priorityMode === 'multiplier'
    && !policy.autoDegradeEnabled
    && policy.modelTargets.length === 0
  ) {
    return 'multiplier_only'
  }
  return 'health_probe'
}

// Include legacy per-channel assignments as well as the group's own bindings.
export const groupAutomationPolicyIds = (group: AdminGroupHealth): string[] => [...new Set([
  ...(group.assignedPolicyIds ?? []),
  ...(group.assignedPolicies ?? []).map(policy => policy.policyId),
  ...group.accounts.flatMap(account => [
    ...(account.assignedPolicyIds ?? []),
    ...(account.assignedPolicies ?? []).map(policy => policy.policyId),
  ]),
])]

export type AutomationCapability = 'monitor' | 'priority' | 'priorityOnly' | 'suspend' | 'localSuspend' | 'unconfigured'

// Describe permitted actions, not just the raw remote-action checkbox. All three
// permissions are required to suspend the upstream channel.
export const automationCapability = (policy: ConnectionHealthPolicy): AutomationCapability => {
  if (resolveConnectionHealthStrategyMode(policy) === 'multiplier_only') return 'priorityOnly'
  if (!policy.modelTargets.some(model => model.enabled && model.modelName.trim())) {
    return policy.priorityMode === 'multiplier' ? 'priorityOnly' : 'unconfigured'
  }
  if (policy.autoDegradeEnabled && policy.autoSuspendEnabled) {
    return policy.autoRemoteActionEnabled ? 'suspend' : 'localSuspend'
  }
  return policy.priorityMode === 'multiplier' || policy.priorityMode === 'latency' ? 'priority' : 'monitor'
}

// Switching a policy must preserve every setting, including suspension permission
// and model prompts. Keep the request independent of read-only response fields.
export const policyInputWithEnabled = (policy: ConnectionHealthPolicy, enabled: boolean): PolicyInput => ({
  id: policy.id,
  name: policy.name,
  enabled,
  ownGroupId: policy.ownGroupId,
  ownGroupName: policy.ownGroupName,
  modelPattern: policy.modelPattern,
  probeMode: (policy.probeMode || 'real_model') as PolicyInput['probeMode'],
  probeIntervalSeconds: policy.probeIntervalSeconds,
  maxLatencyMs: policy.maxLatencyMs ?? 20000,
  failureThreshold: policy.failureThreshold,
  successThreshold: policy.successThreshold,
  dailyProbeBudget: policy.dailyProbeBudget,
  autoDegradeEnabled: policy.autoDegradeEnabled,
  autoRemoteActionEnabled: policy.autoRemoteActionEnabled,
  autoSuspendEnabled: policy.autoSuspendEnabled ?? false,
  priorityMode: policy.priorityMode ?? 'none',
  ...(policy.latencyPriority ? { latencyPriority: JSON.parse(JSON.stringify(policy.latencyPriority)) } : {}),
  strategyMode: resolveConnectionHealthStrategyMode(policy),
  modelTargets: policy.modelTargets.map(model => ({
    id: model.id, modelName: model.modelName, providerFamily: model.providerFamily,
    enabled: model.enabled, probePrompt: model.probePrompt, maxProbeTokens: model.maxProbeTokens,
  })),
})
