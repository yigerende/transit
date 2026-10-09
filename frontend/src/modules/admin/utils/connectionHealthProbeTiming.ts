import type { ConnectionHealthEvent, ModelHealth } from '../types/connectionHealth'

// Manual status changes are events, but do not measure a conversation.
export const PROBE_RESULTS = new Set(['ok', 'network_fluctuation', 'rate_limited', 'server_error', 'auth', 'model_not_found', 'invalid_response', 'unsupported'])

type ProbeEvent = Pick<ConnectionHealthEvent, 'id' | 'createdAt' | 'result' | 'latencyMs'>
type ProbeState = Pick<ModelHealth, 'lastProbeAt' | 'lastLatencyMs' | 'lastErrorKey'>

const validLatency = (latency: number | null): number | null =>
  latency != null && Number.isFinite(latency) && latency >= 0 ? latency : null

export const latestProbeTiming = (events: ProbeEvent[], model?: ProbeState) => {
  const latest = events.reduce<ProbeEvent | undefined>((current, event) => {
    if (!PROBE_RESULTS.has(event.result)) return current
    if (!current) return event
    const difference = Date.parse(event.createdAt) - Date.parse(current.createdAt)
    return difference > 0 || (difference === 0 && event.id > current.id) ? event : current
  }, undefined)

  // The event list and model state refresh separately. Keep the duration and
  // outcome from the same probe, including when a newer failure has no duration.
  if (model?.lastProbeAt && (!latest || Date.parse(model.lastProbeAt) > Date.parse(latest.createdAt))) {
    return { latestLatencyMs: validLatency(model.lastLatencyMs), latestProbeFailed: !!model.lastErrorKey }
  }
  return {
    latestLatencyMs: latest ? validLatency(latest.latencyMs) : null,
    latestProbeFailed: !!latest && latest.result !== 'ok',
  }
}
