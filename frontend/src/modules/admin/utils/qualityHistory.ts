import type { QualityManualMethod, QualitySample } from '../types/quality'

export const qualitySampleMethod = (sample: QualitySample): QualityManualMethod =>
  sample.detectionMethod === 'manxue' ? sample.benchmark === 'pelican' ? 'manxue_pelican' : 'manxue_candy' : 'questions'

export const qualityHistoryNewestFirst = (samples: QualitySample[]): QualitySample[] =>
  [...samples].sort((a, b) => Date.parse(b.startedAt || b.createdAt) - Date.parse(a.startedAt || a.createdAt) || b.id.localeCompare(a.id))

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
