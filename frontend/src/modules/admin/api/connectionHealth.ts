import type { QualitySettings, QualityGroup, QualityChannel, QualitySample, QualityManualMethod } from '../types/quality'
import type {
  AdminGroupPolicyConfiguration,
  AdminGroupPolicyConfigurationInput,
  AdminGroupHealth,
  ConnectionHealthEvent,
  ConnectionHealthOverview,
  ConnectionHealthStoredSummary,
  ConnectionHealthPolicy,
  ManualProbeModelOption,
  ManualProbeResult,
  ModelHealth,
  OwnGroupHealth,
  PolicyInput,
  TargetPolicyAssignments,
  GroupProbeSample,
  GroupProbeConfig,
} from '../types/connectionHealth'
import {
  authUnauthorizedErrorKey,
  getAccessToken,
  handleAuthExpired,
  isUnauthorizedApiResponse,
} from '@/modules/auth/api/auth'

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? '/api'

export const getQualitySettings = (): Promise<QualitySettings> => requestJson('/connection-health/quality-settings')
export const saveQualitySettings = (config: QualitySettings): Promise<QualitySettings> => requestJson('/connection-health/quality-settings', { method: 'PUT', body: JSON.stringify(config) })
export const setGroupQuality = (groupId: string, enabled: boolean): Promise<QualityGroup> => requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/quality`, { method: 'PUT', body: JSON.stringify({ enabled }) })

export const setChannelQuality = (targetId: string, enabled: boolean): Promise<QualityChannel> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/quality`, { method: 'PUT', body: JSON.stringify({ enabled }) })
export const probeChannelQuality = (targetId: string, groupId: string, method: QualityManualMethod): Promise<QualitySample> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/quality-probe`, { method: 'POST', body: JSON.stringify({ groupId, method }) })
export const getChannelQualityHistory = (targetId: string): Promise<QualitySample[]> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/quality-history`)
export const getChannelQualityDetail = (targetId: string, sampleId: string): Promise<QualitySample> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/quality-history/${encodeURIComponent(sampleId)}`)
export const setChannelSuspension = (targetId: string, enabled: boolean): Promise<{ targetId: string; enabled: boolean }> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/suspension`, { method: 'PUT', body: JSON.stringify({ enabled }) })

const endpoint = (path: string): string => `${apiBaseUrl.replace(/\/$/, '')}${path}`

const authHeaders = (): HeadersInit => {
  const token = getAccessToken()
  if (!token) return {}
  return { Authorization: `Bearer ${token}` }
}

type ApiErrorPayload = {
  message?: string
}

const requestJson = async <T>(path: string, options: RequestInit = {}): Promise<T> => {
  let response: Response
  try {
    response = await fetch(endpoint(path), {
      ...options,
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        ...authHeaders(),
        ...(options.headers ?? {}),
      },
    })
  } catch {
    throw new Error('admin.connectionHealth.errors.network')
  }

  const text = await response.text()
  const payload = text ? (JSON.parse(text) as T & ApiErrorPayload) : ({} as T & ApiErrorPayload)

  if (!response.ok) {
    if (isUnauthorizedApiResponse(response.status, payload)) {
      handleAuthExpired()
      throw new Error(authUnauthorizedErrorKey)
    }
    throw new Error(payload.message ?? 'admin.connectionHealth.errors.request')
  }

  return payload
}

export const getConnectionHealthOverview = async (): Promise<ConnectionHealthOverview> =>
  requestJson<ConnectionHealthOverview>('/connection-health/overview')

export const getConnectionHealthStoredSummary = async (): Promise<ConnectionHealthStoredSummary> =>
  requestJson<ConnectionHealthStoredSummary>('/connection-health/stored-summary')

export const getConnectionHealthGroups = async (): Promise<OwnGroupHealth[]> =>
  requestJson<OwnGroupHealth[]>('/connection-health/groups')

// getConnectionHealthAdminGroups 拉取 admin 全量分组健康主列表（含账号/渠道与探活叠加）。
// 对应后端新增路由，不影响旧的 getConnectionHealthGroups。
export const getConnectionHealthAdminGroups = async (): Promise<AdminGroupHealth[]> =>
  requestJson<AdminGroupHealth[]>('/connection-health/admin-groups')

export const getConnectionHealthGroupSummaries = (): Promise<AdminGroupHealth[]> =>
  requestJson('/connection-health/admin-groups?view=summary')

export const getConnectionHealthGroupDetail = (groupId: string): Promise<AdminGroupHealth> =>
  requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/health`)

export const getConnectionHealthEvents = async (connectionId?: string, limit = 100): Promise<ConnectionHealthEvent[]> => {
  const params = new URLSearchParams()
  if (connectionId) params.set('connectionId', connectionId)
  params.set('limit', String(limit))
  return requestJson<ConnectionHealthEvent[]>(`/connection-health/events?${params.toString()}`)
}

// probeConnection 触发一次手动探活。models 为空或未传时保持旧行为：探活该连接匹配到的
// 全部启用模型目标（向后兼容旧调用方）。传入 models 时后端只探活这些模型名对应的匹配目标。
export const probeConnection = async (connectionId: string, models?: string[]): Promise<ModelHealth[]> =>
  requestJson<ModelHealth[]>(`/connection-health/connections/${encodeURIComponent(connectionId)}/probe`, {
    method: 'POST',
    body: models && models.length > 0 ? JSON.stringify({ models }) : undefined,
  })

// probeTarget 触发一次独立目标的手动探活：按 targetId + models 走，后端 server-only 解析凭据，
// 请求体绝不携带 base_url/key/credentials。models 为空时后端探活该目标全部候选模型。
export const probeTarget = async (targetId: string, models?: string[]): Promise<ModelHealth[]> =>
  requestJson<ModelHealth[]>(`/connection-health/targets/${encodeURIComponent(targetId)}/probe`, {
    method: 'POST',
    body: models && models.length > 0 ? JSON.stringify({ models }) : undefined,
  })

// discoverTargetModels 是手动一次性探活弹窗打开时调用的 server-only 模型发现接口：
// 后端用当前 admin session 临时解析该 target 的 base_url + key 请求上游 /v1/models，
// 这里只拿到安全字段（id/name/ownedBy/providerFamily），前端绝不接触凭据本身。
export const discoverTargetModels = async (targetId: string): Promise<ManualProbeModelOption[]> =>
  requestJson<ManualProbeModelOption[]>(`/connection-health/targets/${encodeURIComponent(targetId)}/models`)

// 手动探活保存渠道历史，不修改策略状态或触发远端动作。
// models 必须非空——手动一次性探活没有候选池概念，必须由用户在弹窗里显式勾选。
export const manualProbeOnce = async (targetId: string, models: string[]): Promise<ManualProbeResult[]> =>
  requestJson<ManualProbeResult[]>(`/connection-health/targets/${encodeURIComponent(targetId)}/manual-probe`, {
    method: 'POST',
    body: JSON.stringify({ models, recordHistory: true }),
  })

export type GroupProbeKeyInput = { key?: string; useAutoKey?: boolean }

export const prepareGroupProbe = (groupId: string, selection: GroupProbeKeyInput): Promise<{ models: ManualProbeModelOption[]; modelListUnavailable: boolean }> =>
  requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/prepare-probe`, { method: 'POST', body: JSON.stringify(selection) })

export const getGroupProbeConfig = (groupId: string): Promise<GroupProbeConfig | null> =>
  requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/probe-config`)

export const saveGroupProbeConfig = (groupId: string, config: Pick<GroupProbeConfig, 'model' | 'probeMode' | 'intervalSeconds' | 'enabled'> & GroupProbeKeyInput): Promise<GroupProbeConfig> =>
  requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/probe-config`, { method: 'PUT', body: JSON.stringify(config) })

export const probeAdminGroup = (groupId: string, model: string): Promise<GroupProbeSample> =>
  requestJson(`/connection-health/admin-groups/${encodeURIComponent(groupId)}/probe`, { method: 'POST', body: JSON.stringify({ model }) })

// getTargetPolicyAssignments / setTargetPolicyAssignments 管理「账号/channel 显式分配策略」
// 关系：只有分配了已启用策略的 target，后台调度器才会自动探活。
export const getTargetPolicyAssignments = async (targetId: string): Promise<TargetPolicyAssignments> =>
  requestJson<TargetPolicyAssignments>(`/connection-health/targets/${encodeURIComponent(targetId)}/policy-assignments`)

export const setTargetPolicyAssignments = async (targetId: string, policyIds: string[]): Promise<TargetPolicyAssignments> =>
  requestJson<TargetPolicyAssignments>(`/connection-health/targets/${encodeURIComponent(targetId)}/policy-assignments`, {
    method: 'PUT',
    body: JSON.stringify({ policyIds }),
  })

export const getAdminGroupPolicyConfiguration = async (adminGroupId: string): Promise<AdminGroupPolicyConfiguration> =>
  requestJson<AdminGroupPolicyConfiguration>(
    `/connection-health/admin-groups/${encodeURIComponent(adminGroupId)}/policy-configuration`,
  )

export const setAdminGroupPolicyConfiguration = async (
  adminGroupId: string,
  input: AdminGroupPolicyConfigurationInput,
): Promise<AdminGroupPolicyConfiguration> =>
  requestJson<AdminGroupPolicyConfiguration>(
    `/connection-health/admin-groups/${encodeURIComponent(adminGroupId)}/policy-configuration`,
    { method: 'PUT', body: JSON.stringify(input) },
  )

export const disableConnection = async (connectionId: string): Promise<void> => {
  await requestJson<{ ok: boolean }>(`/connection-health/connections/${encodeURIComponent(connectionId)}/disable`, {
    method: 'POST',
  })
}

export const restoreConnection = async (connectionId: string): Promise<void> => {
  await requestJson<{ ok: boolean }>(`/connection-health/connections/${encodeURIComponent(connectionId)}/restore`, {
    method: 'POST',
  })
}

export const listConnectionHealthPolicies = async (): Promise<ConnectionHealthPolicy[]> =>
  requestJson<ConnectionHealthPolicy[]>('/connection-health/policies')

export const createConnectionHealthPolicy = async (input: PolicyInput): Promise<ConnectionHealthPolicy> =>
  requestJson<ConnectionHealthPolicy>('/connection-health/policies', {
    method: 'POST',
    body: JSON.stringify(input),
  })

export const updateConnectionHealthPolicy = async (id: string, input: PolicyInput): Promise<ConnectionHealthPolicy> =>
  requestJson<ConnectionHealthPolicy>(`/connection-health/policies/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })

export const deleteConnectionHealthPolicy = async (id: string): Promise<void> => {
  await requestJson<{ ok: boolean }>(`/connection-health/policies/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export const setChannelPriority = (targetId: string, enabled: boolean): Promise<{ targetId: string; enabled: boolean; restorePending: boolean; priority: number | null }> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/priority`, { method: 'PUT', body: JSON.stringify({ enabled }) })

export const setChannelAutoProbe = (targetId: string, enabled: boolean): Promise<{ targetId: string; enabled: boolean }> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/auto-probe`, { method: 'PUT', body: JSON.stringify({ enabled }) })

export const setChannelQualitySuspension = (targetId: string, enabled: boolean): Promise<{ targetId: string; enabled: boolean; restorePending: boolean }> => requestJson(`/connection-health/targets/${encodeURIComponent(targetId)}/quality-suspension`, { method: 'PUT', body: JSON.stringify({ enabled }) })
