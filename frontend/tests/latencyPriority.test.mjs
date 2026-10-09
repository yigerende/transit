import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

const load = async name => {
  const source = await readFile(new URL(`../src/modules/admin/utils/${name}.ts`, import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
}
const { defaultLatencyPriority, validLatencyPriority } = await load('latencyPriority')
const { policyInputWithEnabled, automationCapability } = await load('connectionHealthPolicy')

test('latency settings survive group switches with independent copies', () => {
  const config = defaultLatencyPriority()
  config.sampleCount = 2; config.weights = [3, 1]; config.maxAgeSeconds = 240
  config.bands[0].priority = 7
  const policy = { id: 'p', name: 'latency', enabled: true, priorityMode: 'latency', strategyMode: 'health_probe',
    latencyPriority: config, autoDegradeEnabled: true, autoRemoteActionEnabled: false, autoSuspendEnabled: false,
    modelTargets: [{ modelName: 'model', enabled: true }],
  }
  assert.equal(automationCapability(policy), 'priority')
  for (const enabled of [true, false]) {
    const saved = policyInputWithEnabled(policy, enabled)
    assert.deepEqual(saved.latencyPriority, config)
    assert.equal(saved.priorityMode, 'latency')
    assert.equal(saved.enabled, enabled)
    saved.latencyPriority.bands[0].priority = 99
    assert.equal(config.bands[0].priority, 7)
  }
})

test('sample and band validation rejects ambiguous priority mappings', () => {
  assert.equal(validLatencyPriority(defaultLatencyPriority()), true)
  for (const mutate of [
    c => { c.sampleCount = 0 }, c => { c.sampleCount = 21 }, c => { c.sampleCount = 2.5 },
    c => { c.minSamples = 4 }, c => { c.maxAgeSeconds = 0 }, c => { c.weights = [1] },
    c => { c.weights[0] = 0 }, c => { c.weights[0] = NaN }, c => { c.hysteresisSeconds = -1 },
    c => { c.bands[1].minSeconds = 4 }, c => { c.bands[1].minSeconds = 6 },
    c => { c.bands[3].maxSeconds = 20 }, c => { c.bands[1].priority = 1 },
    c => { c.bands[1].maxSeconds = null }, c => { c.suspendedPriority = -1 },
  ]) {
    const config = defaultLatencyPriority(); mutate(config)
    assert.equal(validLatencyPriority(config), false, JSON.stringify(config))
  }
})
