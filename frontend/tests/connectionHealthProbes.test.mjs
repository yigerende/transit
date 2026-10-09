import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

const load = async (name) => {
  const source = await readFile(new URL(`../src/modules/admin/utils/${name}.ts`, import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
}
const { channelsByLatestLatency, channelAutomationEnabled } = await load('connectionHealthChannels')
const { latestProbeTiming } = await load('connectionHealthProbeTiming')

const sample = (result, latencyMs, minute = 2, id = 'probe') => ({
  id, result, latencyMs, createdAt: `2026-10-09T09:${String(minute).padStart(2, '0')}:00Z`,
})
const channel = (name, recentProbes, extra = {}) => ({ name, recentProbes, hasEnabledPolicy: true, ...extra })

test('quick failures cannot outrank successful conversations, even with an earlier fast success', () => {
  const accounts = [
    channel('fast error', [sample('server_error', 1043), sample('ok', 100, 1)]),
    channel('slower success', [sample('ok', 9600)]),
    channel('timeout', [sample('network_fluctuation', 20000)]),
    channel('fast success', [sample('ok', 5300)]),
  ]
  assert.deepEqual(channelsByLatestLatency(accounts).map(a => a.name), ['fast success', 'slower success', 'fast error', 'timeout'])
  assert.equal(accounts[0].name, 'fast error', 'sorting must not mutate API data')
})

test('sorting uses the newest result, preserves ties, and keeps inactive channels in their own section', () => {
  const accounts = [
    channel('unprobed', []),
    channel('equal first', [sample('ok', 6000), sample('ok', 200, 1)]),
    channel('equal second', [sample('ok', 6000)]),
    channel('recovered', [sample('server_error', 100, 1), sample('ok', 5000)]),
    channel('missing duration', [sample('ok', null)]),
    channel('inactive', [sample('ok', 1)], { hasEnabledPolicy: false }),
    channel('excluded', [sample('ok', 2)], { excludedFromGroupPolicy: true }),
  ]
  assert.deepEqual(channelsByLatestLatency(accounts.filter(channelAutomationEnabled)).map(a => a.name),
    ['recovered', 'equal first', 'equal second', 'missing duration', 'unprobed'])
})

test('every failure result reports failure duration, never chat latency', () => {
  for (const result of ['network_fluctuation', 'rate_limited', 'server_error', 'auth', 'model_not_found', 'invalid_response', 'unsupported']) {
    assert.deepEqual(latestProbeTiming([sample(result, 1043)]), { latestLatencyMs: 1043, latestProbeFailed: true })
  }
  assert.deepEqual(latestProbeTiming([sample('ok', 5300)]), { latestLatencyMs: 5300, latestProbeFailed: false })
})

test('manual actions and unordered events do not replace the most recent probe duration or outcome', () => {
  assert.deepEqual(latestProbeTiming([
    sample('ok', 1200, 1), sample('disabled', 0, 3), sample('server_error', 1043),
  ]), { latestLatencyMs: 1043, latestProbeFailed: true })
  assert.deepEqual(latestProbeTiming([sample('disabled', 0)]), { latestLatencyMs: null, latestProbeFailed: false })
})

test('separately refreshed state and history keep duration paired with its own outcome', () => {
  const state = { lastProbeAt: sample('ok', 0, 3).createdAt, lastLatencyMs: 274, lastErrorKey: 'server_error' }
  assert.deepEqual(latestProbeTiming([sample('ok', 5000)], state), { latestLatencyMs: 274, latestProbeFailed: true })
  assert.deepEqual(latestProbeTiming([sample('ok', 6000, 4)], state), { latestLatencyMs: 6000, latestProbeFailed: false })
  assert.deepEqual(latestProbeTiming([sample('server_error', null, 3)], { ...state, lastLatencyMs: 5000, lastErrorKey: '' }),
    { latestLatencyMs: null, latestProbeFailed: true })
  assert.deepEqual(latestProbeTiming([], state), { latestLatencyMs: 274, latestProbeFailed: true })
  assert.deepEqual(latestProbeTiming([], { ...state, lastProbeAt: null }), { latestLatencyMs: null, latestProbeFailed: false })
})
