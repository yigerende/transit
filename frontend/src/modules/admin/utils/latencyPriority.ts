import type { LatencyPriorityConfig } from '../types/connectionHealth'

export const defaultLatencyWeights = (size: number): number[] =>
  size === 5 ? [30, 25, 20, 15, 10] : size === 3 ? [50, 30, 20] : Array.from({ length: size }, (_, i) => size - i)

export const defaultLatencyPriority = (): LatencyPriorityConfig => ({
  modelName: '', sampleCount: 5, minSamples: 1, maxAgeSeconds: 300,
  weights: defaultLatencyWeights(5), hysteresisSeconds: 0.5,
  bands: [
    { minSeconds: 0, maxSeconds: 5, priority: 10 },
    { minSeconds: 5, maxSeconds: 10, priority: 200 },
    { minSeconds: 10, maxSeconds: 15, priority: 400 },
    { minSeconds: 15, maxSeconds: null, priority: 600 },
  ],
  insufficientPriority: 50, degradedPriority: 100, suspendedPriority: 10000,
})

export const validLatencyPriority = (c: LatencyPriorityConfig): boolean => {
  const integer = (value: number, min: number, max: number) => Number.isInteger(value) && value >= min && value <= max
  if (!integer(c.sampleCount, 1, 20) || !integer(c.minSamples, 1, c.sampleCount) || !integer(c.maxAgeSeconds, 1, 86400)
    || c.weights.length !== c.sampleCount || c.weights.some(w => !Number.isFinite(w) || w <= 0 || w > 1000000)
    || !Number.isFinite(c.hysteresisSeconds) || c.hysteresisSeconds < 0 || c.hysteresisSeconds > 3600
    || c.bands.length < 1 || c.bands.length > 20
    || [c.insufficientPriority, c.degradedPriority, c.suspendedPriority].some(p => !integer(p, 0, 2147483647))) return false
  let end = 0
  const priorities = new Set<number>()
  for (const [i, band] of c.bands.entries()) {
    if (!Number.isFinite(band.minSeconds) || band.minSeconds !== end || !integer(band.priority, 0, 2147483647) || priorities.has(band.priority)) return false
    priorities.add(band.priority)
    if (band.maxSeconds === null) return i === c.bands.length - 1
    if (!Number.isFinite(band.maxSeconds) || band.maxSeconds <= end || band.maxSeconds > 86400) return false
    end = band.maxSeconds
  }
  return false
}
