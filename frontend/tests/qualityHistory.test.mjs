import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

const source = await readFile(new URL('../src/modules/admin/utils/qualityHistory.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
const { qualitySampleTime, qualityFormatTime, qualityHistoryNewestFirst, qualityEstimatedCostUSD, qualityFormatCost } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
const sample = { detectionMethod: 'manxue', model: 'gpt-6-astra', inputTokens: 4317, outputTokens: 949, serviceTier: 'default' }

test('reference estimate matches the benchmark site and respects service tiers', () => {
  assert.equal(qualityFormatCost(qualityEstimatedCostUSD(sample)), '$0.0906')
  for (const [tier, multiplier] of [['default', 1], ['priority', 2], ['fast', 2], ['flex', 0.5], ['ultrafast', 6]]) {
    assert.equal(qualityEstimatedCostUSD({ ...sample, serviceTier: tier }), 0.09062 * multiplier)
  }
  assert.equal(qualityEstimatedCostUSD({ ...sample, inputTokens: 272000, outputTokens: 1000 }), 2.77)
  assert.equal(qualityEstimatedCostUSD({ ...sample, inputTokens: 272001, outputTokens: 1000 }), 5.51502)
  assert.equal(qualityEstimatedCostUSD({ ...sample, inputTokens: 0, outputTokens: 0, usageReported: true }), 0)
  assert.equal(qualityFormatCost(0.000001), '<$0.0001')
})

test('missing, invalid or unsupported usage cannot become a misleading cost', () => {
  for (const change of [
    { inputTokens: undefined }, { outputTokens: undefined }, { inputTokens: -1 }, { inputTokens: 1.5 },
    { outputTokens: NaN }, { inputTokens: Infinity }, { inputTokens: Number.MAX_SAFE_INTEGER + 1 },
    { usageReported: false }, { model: 'unknown' }, { model: 'constructor' }, { serviceTier: 'unknown' },
    { serviceTier: 'ultrafast', model: 'gpt-6.1-sol' }, { detectionMethod: 'questions' },
    { inputTokens: 0, outputTokens: 0 },
  ]) assert.equal(qualityEstimatedCostUSD({ ...sample, ...change }), undefined)
  assert.equal(qualityFormatCost(undefined), '—')
  assert.equal(qualityEstimatedCostUSD({ ...sample, serviceTier: undefined }), 0.09062)
})

test('timeline and detail use start time, keeping legacy completion times as fallback', () => {
  const older = { id: 'older', startedAt: '2026-11-12T17:20:52Z', createdAt: '2026-11-12T17:30:30Z' }
  const newer = { id: 'newer', startedAt: '2026-11-12T17:28:07Z', createdAt: '2026-11-12T17:30:24Z' }
  const legacy = { id: 'legacy', createdAt: '2026-11-12T17:22:00Z' }
  assert.deepEqual(qualityHistoryNewestFirst([older, legacy, newer]).map(s => s.id), ['newer', 'legacy', 'older'])
  assert.equal(qualitySampleTime(newer), newer.startedAt)
  assert.equal(qualitySampleTime(legacy), legacy.createdAt)
  assert.equal(qualityFormatTime(newer.startedAt, 'zh-CN'), new Date(newer.startedAt).toLocaleString('zh-CN', { hour12: false }))
  assert.equal(qualityFormatTime(undefined, 'zh-CN'), '—')
  assert.equal(qualityFormatTime('invalid', 'zh-CN'), '—')
})
