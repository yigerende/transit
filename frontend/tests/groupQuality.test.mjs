import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'
const source = await readFile(new URL('../src/modules/group-quality/stats.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
const { verdictRate, healthAverage, samplesFor } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
test('quality request errors are excluded from verdict accuracy', () => {
  assert.deepEqual(verdictRate([{ result: 'passed' }, { result: 'failed' }, { result: 'error' }]), { passed: 1, total: 2, errors: 1, percent: 50 })
  assert.equal(verdictRate([{ result: 'error' }]).percent, undefined)
})
test('latency uses five newest successes of the currently selected method', () => {
  const card = { monitor: { health: { probeMode: 'first_token' } }, history: [] }
  for (let i = 1; i <= 6; i++) card.history.push({ id: String(i), kind: 'health', result: 'passed', probeMode: 'first_token', durationMs: i * 1000, startedAt: `2026-11-13T01:0${i}:00Z` })
  card.history.push({ id: 'wrong-mode', kind: 'health', result: 'passed', probeMode: 'real_model', durationMs: 999000, startedAt: '2026-11-13T01:07:00Z' })
  card.history.push({ id: 'failed', kind: 'health', result: 'error', probeMode: 'first_token', durationMs: 20000, startedAt: '2026-11-13T01:08:00Z' })
  assert.equal(healthAverage(card), 4)
  assert.equal(samplesFor(card, 'health')[0].id, 'failed')
  assert.equal(healthAverage({ ...card, history: [card.history.at(-1)] }), undefined)
})
