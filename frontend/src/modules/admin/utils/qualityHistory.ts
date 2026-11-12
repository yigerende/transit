import type { QualityManualMethod, QualitySample } from '../types/quality'

export const qualitySampleMethod = (sample: QualitySample): QualityManualMethod =>
  sample.detectionMethod === 'manxue' ? sample.benchmark === 'pelican' ? 'manxue_pelican' : 'manxue_candy' : 'questions'

export const qualitySampleTime = (sample: QualitySample): string => sample.startedAt || sample.createdAt

export const qualityFormatTime = (value: string | undefined, locale: string): string => {
  const date = new Date(value || '')
  return Number.isFinite(date.getTime()) ? date.toLocaleString(locale, { hour12: false }) : '—'
}

export const qualityHistoryNewestFirst = (samples: QualitySample[]): QualitySample[] =>
  [...samples].sort((a, b) => Date.parse(qualitySampleTime(b)) - Date.parse(qualitySampleTime(a)) || b.id.localeCompare(a.id))

// Reference estimate matching manxue.ai's public page, checked 2026-11-13.
// USD per million tokens; this is not the channel's billed price.
const referencePrices: Record<string, readonly [number, number]> = {
  'gpt-5.6-luna': [0.2, 1.2], 'gpt-5.6-terra': [2, 12], 'gpt-5.6-sol': [4, 20],
  'gpt-6-luna': [0.1, 0.5], 'gpt-6-sol': [2, 10], 'gpt-6-astra': [10, 50], 'gpt-6.1-sol': [2, 10],
}

export function qualityEstimatedCostUSD(sample: QualitySample): number | undefined {
  if (sample.detectionMethod !== 'manxue' || sample.usageReported === false) return
  const input = sample.inputTokens, output = sample.outputTokens
  if (input == null || output == null || !Number.isSafeInteger(input) || !Number.isSafeInteger(output) || input < 0 || output < 0) return
  if (input === 0 && output === 0 && sample.usageReported !== true) return
  const model = sample.model.trim().toLowerCase()
  if (!Object.hasOwn(referencePrices, model)) return
  const prices = referencePrices[model]!
  const tier = sample.serviceTier?.trim().toLowerCase() || 'default'
  const multiplier = tier === 'default' ? 1 : tier === 'priority' || tier === 'fast' ? 2 : tier === 'flex' ? 0.5 : tier === 'ultrafast' && model === 'gpt-6-astra' ? 6 : undefined
  if (multiplier == null) return
  const longContext = input > 272_000
  return (input * prices[0] * (longContext ? 2 : 1) + output * prices[1] * (longContext ? 1.5 : 1)) * multiplier / 1_000_000
}

export const qualityFormatCost = (cost: number | undefined): string =>
  cost == null ? '—' : cost > 0 && cost < 0.0001 ? '<$0.0001' : new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 4, maximumFractionDigits: 4 }).format(cost)

// Render generated SVG/HTML/canvas in an opaque-origin sandbox. Inline scripts
// can draw canvas; network, child frames, forms, navigation of the app and access
// to its storage are unavailable. Never insert generated markup into the app DOM.
export function qualityPreviewDocument(html: string): string {
  const source = html.trim().replace(/^```(?:html)?\s*\n/i, '').replace(/\n```\s*$/, '')
  const document = new DOMParser().parseFromString(source, 'text/html')
  document.querySelectorAll('meta, base, iframe, frame, frameset, object, embed, link, script[src]').forEach(node => node.remove())
  document.querySelectorAll('a, area, form').forEach(node => {
    for (const name of ['href', 'xlink:href', 'action', 'target', 'ping', 'download']) node.removeAttribute(name)
  })
  const policy = document.createElement('meta')
  policy.httpEquiv = 'Content-Security-Policy'
  policy.content = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"
  document.head.prepend(policy)
  return '<!doctype html>\n' + document.documentElement.outerHTML
}
