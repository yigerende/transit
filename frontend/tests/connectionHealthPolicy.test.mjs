import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

// Run pure policy logic with the project's existing TypeScript compiler, without
// adding a browser/testing framework or requiring Node's experimental TS loader.
const source = await readFile(new URL('../src/modules/admin/utils/connectionHealthPolicy.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } })
const { automationCapability, groupAutomationPolicyIds, policyInputWithEnabled } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)

const policy = (overrides = {}) => ({
  id: 'policy', name: 'Group monitoring', enabled: true, ownGroupId: 'g1', ownGroupName: 'Group',
  modelPattern: 'gpt-*', probeMode: 'real_model', strategyMode: 'health_probe', priorityMode: 'none',
  probeIntervalSeconds: 60, maxLatencyMs: 45000, failureThreshold: 3, successThreshold: 2, cooldownSeconds: 300,
  observationSeconds: 120, recoveryStepPercent: 25, dailyProbeBudget: 1000,
  autoDegradeEnabled: true, autoRemoteActionEnabled: false, autoSuspendEnabled: false,
  modelTargets: [{ id: 'model', modelName: 'gpt-4o', providerFamily: 'openai', enabled: true, probePrompt: 'Custom prompt', maxProbeTokens: 17 }],
  ...overrides,
})

test('upstream suspension label requires all three permissions', () => {
  for (const autoDegradeEnabled of [true, false]) {
    for (const autoRemoteActionEnabled of [true, false]) {
      for (const autoSuspendEnabled of [true, false]) {
        const mode = automationCapability(policy({ autoDegradeEnabled, autoRemoteActionEnabled, autoSuspendEnabled }))
        const expected = autoDegradeEnabled && autoSuspendEnabled
          ? autoRemoteActionEnabled ? 'suspend' : 'localSuspend' : 'monitor'
        assert.equal(mode, expected)
      }
    }
  }
})

test('priority-only and monitor modes reflect actual configuration', () => {
  assert.equal(automationCapability(policy({ priorityMode: 'multiplier' })), 'priority')
  assert.equal(automationCapability(policy({ priorityMode: 'none' })), 'monitor')
  assert.equal(automationCapability(policy({ strategyMode: 'multiplier_only', autoSuspendEnabled: true, autoRemoteActionEnabled: true })), 'priorityOnly')
  assert.equal(automationCapability(policy({ strategyMode: undefined, autoDegradeEnabled: false, priorityMode: 'multiplier', modelTargets: [] })), 'priorityOnly')
  assert.equal(automationCapability(policy({ modelTargets: [] })), 'unconfigured')
  assert.equal(automationCapability(policy({ modelTargets: [{ modelName: 'm', enabled: false }], priorityMode: 'multiplier' })), 'priorityOnly')
})

test('group and legacy channel policies are deduplicated without including unrelated policies', () => {
  assert.deepEqual(groupAutomationPolicyIds({ assignedPolicyIds: ['group'], assignedPolicies: [{ policyId: 'group' }], accounts: [
    { assignedPolicyIds: ['group', 'legacy'] },
    { assignedPolicies: [{ policyId: 'legacy' }, { policyId: 'other-model' }] },
    { excludedFromGroupPolicy: true },
  ] }), ['group', 'legacy', 'other-model'])
  assert.deepEqual(groupAutomationPolicyIds({ accounts: [] }), [])
})

test('toggle preserves suspension permission, thresholds and model configuration in both directions', () => {
  const original = policy({ autoSuspendEnabled: true, autoRemoteActionEnabled: true })
  const off = policyInputWithEnabled(original, false)
  assert.equal(off.enabled, false)
  for (const [key, value] of Object.entries(original)) {
    if (key !== 'enabled' && key !== 'probeMode') assert.deepEqual(off[key], value, key)
  }
  assert.deepEqual(policyInputWithEnabled({ ...original, enabled: false }, true), { ...off, enabled: true })
  assert.notEqual(off.modelTargets, original.modelTargets)
  assert.equal(original.enabled, true)
  assert.equal(policyInputWithEnabled(policy({ maxLatencyMs: undefined }), false).maxLatencyMs, 20000)
  assert.equal(policyInputWithEnabled(policy({ autoSuspendEnabled: undefined }), false).autoSuspendEnabled, false)
})
