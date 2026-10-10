import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import test from 'node:test'
import { effectScope } from 'vue'
import ts from 'typescript'

const require = createRequire(import.meta.url)
const vueURL = pathToFileURL(require.resolve('vue/dist/vue.cjs.js')).href
const source = await readFile(new URL('../src/modules/admin/composables/useGroupHealthPage.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
let fixtureSequence = 0
const turn = () => new Promise(resolve => setImmediate(resolve))
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b }); return { promise, resolve, reject } }
const group = (id, extra = {}) => ({ id, name: id, accounts: [], accountsLoaded: true, ...extra })

async function fixture() {
  const requests = [], summaryRequests = []
  const api = {
    getConnectionHealthGroupSummaries: () => { const d = deferred(); summaryRequests.push(d); return d.promise },
    getConnectionHealthGroupDetail: id => { const d = deferred(); requests.push({ id, ...d }); return d.promise },
  }
  const key = `__groupHealthLoading${fixtureSequence++}`
  globalThis[key] = api
  const js = compiled.replace("from 'vue'", `from '${vueURL}'`).replace(/import \{[^}]+\} from '\.\.\/api\/connectionHealth';/, `const {getConnectionHealthGroupDetail,getConnectionHealthGroupSummaries}=globalThis.${key};`)
  const { useGroupHealthPage } = await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`)
  delete globalThis[key]
  const selected = { id: 'g1' }
  const scope = effectScope()
  const page = scope.run(() => useGroupHealthPage(() => selected.id))
  return { page, selected, scope, requests, summaryRequests }
}

test('sidebar renders before details, histories load for selected groups, and reversed replies stay with their group', async () => {
  const f = await fixture()
  const loading = f.page.loadAll()
  f.summaryRequests[0].resolve([group('g1'), group('g2')])
  await turn()
  assert.equal(f.page.adminGroups.value.length, 2)
  assert.deepEqual(f.requests.map(r => r.id), ['g1'])
  f.selected.id = 'g2'
  const second = f.page.loadGroupDetail('g2')
  f.requests[1].resolve(group('g2', { accounts: [{ targetId: 'second', recentProbes: [{ id: 'new' }] }] }))
  await second
  f.requests[0].resolve(group('g1', { accounts: [{ targetId: 'first', recentProbes: [{ id: 'old' }] }] }))
  await loading
  assert.equal(f.page.adminGroups.value.find(g => g.id === 'g2').accounts[0].targetId, 'second')
  assert.equal(f.page.adminGroups.value.find(g => g.id === 'g1').accounts[0].targetId, 'first')
  f.scope.stop()
})

test('refresh preserves charts while loading and after failure, then retries successfully', async () => {
  const f = await fixture()
  const initial = f.page.loadAll()
  f.summaryRequests[0].resolve([group('g1')]); await turn()
  f.requests[0].resolve(group('g1', { accounts: [{ targetId: 'one', recentProbes: [{ id: 'history' }] }] })); await initial
  const refresh = f.page.loadAll()
  f.summaryRequests[1].resolve([group('g1')]); await turn()
  assert.equal(f.page.adminGroups.value[0].accounts[0].recentProbes[0].id, 'history')
  f.requests[1].reject(new Error('admin.connectionHealth.errors.network')); await refresh
  assert.equal(f.page.adminGroups.value[0].accounts[0].recentProbes[0].id, 'history')
  assert.equal(f.page.detailErrors.value.g1, 'admin.connectionHealth.errors.network')
  const retry = f.page.loadGroupDetail('g1')
  f.requests[2].resolve(group('g1', { accounts: [{ targetId: 'one', recentProbes: [{ id: 'fresh' }] }] })); await retry
  assert.equal(f.page.adminGroups.value[0].accounts[0].recentProbes[0].id, 'fresh')
  assert.equal(f.page.detailErrors.value.g1, undefined)
  f.scope.stop()
})

test('concurrent requests coalesce and a refresh after a mutation waits for a fresh response', async () => {
  const f = await fixture()
  const first = f.page.loadGroupDetail('g1')
  const duplicate = f.page.loadGroupDetail('g1')
  assert.equal(f.requests.length, 1)
  const forced = f.page.loadGroupDetail('g1', true)
  const forcedAgain = f.page.loadGroupDetail('g1', true)
  f.requests[0].resolve(group('g1')); await first; await duplicate; await turn()
  assert.equal(f.requests.length, 2)
  f.requests[1].resolve(group('g1')); await forced; await forcedAgain
  f.scope.stop()
})

test('unmount isolates cached data and late responses from the next workspace', async () => {
  const old = await fixture()
  const pending = old.page.loadAll()
  old.scope.stop()
  const next = await fixture()
  old.summaryRequests[0].resolve([group('old-workspace')]); await pending
  assert.equal(old.page.adminGroups.value.length, 0)
  assert.equal(next.page.adminGroups.value.length, 0)
  assert.equal(old.requests.length, 0)
  next.scope.stop()
})

test('manual refresh shows loading while queued behind a silent refresh', async () => {
  const f = await fixture()
  const background = f.page.loadAll({ silent: true })
  const manual = f.page.loadAll({ force: true })
  assert.equal(f.page.isLoading.value, true)
  f.summaryRequests[0].resolve([group('g1')]); await turn()
  f.requests[0].resolve(group('g1')); await background; await turn()
  assert.equal(f.page.isLoading.value, true)
  assert.equal(f.summaryRequests.length, 2)
  f.summaryRequests[1].resolve([group('g1')]); await turn()
  f.requests[1].resolve(group('g1')); await manual
  assert.equal(f.page.isLoading.value, false)
  f.scope.stop()
})
