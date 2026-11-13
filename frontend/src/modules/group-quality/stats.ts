import type { Card, CheckKind, Sample } from './types'
export const samplesFor = (card: Card, kind: CheckKind): Sample[] => card.history.filter(s => s.kind === kind).sort((a, b) => Date.parse(b.startedAt || b.createdAt) - Date.parse(a.startedAt || a.createdAt) || b.id.localeCompare(a.id))
export function verdictRate(samples: Sample[]) {
  const valid = samples.filter(s => s.result === 'passed' || s.result === 'failed')
  const passed = valid.filter(s => s.result === 'passed').length
  return { passed, total: valid.length, errors: samples.length - valid.length, percent: valid.length ? Math.round(1000 * passed / valid.length) / 10 : undefined }
}
export function healthAverage(card: Card) {
  const samples = samplesFor(card, 'health').filter(s => s.probeMode === card.monitor.health.probeMode && s.result === 'passed').slice(0, 5)
  return samples.length ? samples.reduce((sum, s) => sum + s.durationMs, 0) / samples.length / 1000 : undefined
}
export const resultLabel = (sample: Sample) => sample.result === 'passed' ? '通过' : sample.result === 'failed' ? '未通过' : '请求异常'
export const timeLabel = (time?: string) => time ? new Date(time).toLocaleString('zh-CN', { hour12: false }) : '—'
export const busy = (card: Card, kind: CheckKind) => card.jobs.some(j => j.kind === kind && (j.queued || !!j.runningUntil))
