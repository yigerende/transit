import { getAccessToken, handleAuthExpired } from '../auth/api/auth'
import type { Board, BoardSettings, CheckKind, Group, Monitor, Sample } from './types'
const base = (import.meta.env.VITE_API_BASE_URL ?? '/api').replace(/\/$/, '')
async function request<T>(path: string, options: RequestInit = {}, publicToken?: string): Promise<T> {
  const response = await fetch(base + path, { ...options, headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${publicToken ?? getAccessToken() ?? ''}`, ...options.headers } })
  const payload = await response.json().catch(() => ({ message: `请求失败 (${response.status})` }))
  if (!response.ok) {
    if (publicToken == null && response.status === 401) handleAuthExpired()
    throw new Error(payload.message || '检测请求失败')
  }
  return payload as T
}
const root = '/group-quality'
export const getBoard = (token?: string): Promise<Board> => request(token == null ? root + '/board' : '/embed/group-quality', {}, token)
export const getSettings = (): Promise<BoardSettings> => request(root + '/settings')
export const saveSettings = (config: BoardSettings): Promise<BoardSettings> => request(root + '/settings', { method: 'PUT', body: JSON.stringify(config) })
export const rotateToken = (): Promise<BoardSettings> => request(root + '/rotate-token', { method: 'POST' })
export const getGroups = (): Promise<Group[]> => request(root + '/groups')
export const getModels = (id: string): Promise<{ id: string }[]> => request(`${root}/groups/${encodeURIComponent(id)}/models`, { method: 'POST' })
export const saveMonitor = (monitor: Monitor): Promise<Monitor> => request(root + '/monitors' + (monitor.id ? '/' + encodeURIComponent(monitor.id) : ''), { method: monitor.id ? 'PUT' : 'POST', body: JSON.stringify(monitor) })
export const deleteMonitor = (id: string): Promise<unknown> => request(`${root}/monitors/${encodeURIComponent(id)}`, { method: 'DELETE' })
export const runCheck = (id: string, kind: CheckKind): Promise<unknown> => request(`${root}/monitors/${encodeURIComponent(id)}/run/${kind}`, { method: 'POST' })
export const getSample = (id: string, token?: string): Promise<Sample> => request(`${token == null ? root : '/embed/group-quality'}/samples/${encodeURIComponent(id)}`, {}, token)
