import type { QualitySample } from '../admin/types/quality'
export type CheckKind = 'health' | 'candy' | 'pelican'
export interface CheckConfig {
  enabled: boolean
  intervalSeconds: number
  timeoutSeconds: number
  probeMode?: 'real_model' | 'arithmetic' | 'first_token'
  reasoningEffort?: string
  serviceTier?: string
}
export interface Monitor {
  id: string
  groupId: string
  groupName: string
  name: string
  model: string
  qualityBaseUrl: string
  enabled: boolean
  health: CheckConfig
  candy: CheckConfig
  pelican: CheckConfig
}
export interface Sample extends QualitySample { kind: CheckKind; probeMode?: string }
export interface Job { monitorId: string; kind: CheckKind; queued: boolean; startedAt?: string; runningUntil?: string; nextRunAt: string }
export interface Card { monitor: Monitor; history: Sample[]; jobs: Job[] }
export interface Board { title: string; cards: Card[]; updatedAt: string }
export interface BoardSettings { title: string; enabled: boolean; token: string; visibleMonitorIds: string[] }
export interface Group { id: string; name: string; platform: string }
export const kinds: CheckKind[] = ['health', 'candy', 'pelican']
export const labels: Record<CheckKind, string> = { health: '探活', candy: 'API 糖果', pelican: 'API 鹈鹕' }
export const modeLabels: Record<string, string> = { real_model: '轻量请求', arithmetic: '算术检测', first_token: '流式首字' }
export const defaultMonitor = (): Monitor => ({
  id: '', groupId: '', groupName: '', name: '', model: 'gpt-6-astra', qualityBaseUrl: '', enabled: true,
  health: { enabled: false, intervalSeconds: 60, timeoutSeconds: 20, probeMode: 'first_token' },
  candy: { enabled: false, intervalSeconds: 300, timeoutSeconds: 180, reasoningEffort: 'low', serviceTier: '' },
  pelican: { enabled: false, intervalSeconds: 1800, timeoutSeconds: 600, reasoningEffort: 'low', serviceTier: '' },
})
