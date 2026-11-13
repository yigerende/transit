<script setup lang="ts">
import { computed, onBeforeMount, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Plus, RefreshCw, Radio, Code2, Trash2, Copy, ExternalLink, Search, ArrowUpRight } from 'lucide-vue-next'
import MonitorCard from './MonitorCard.vue'
import MonitorForm from './MonitorForm.vue'
import MonitorDialog from './MonitorDialog.vue'
import MonitorDetail from './MonitorDetail.vue'
import type { Board, BoardSettings, Monitor, Sample, CheckKind } from './types'
import { defaultMonitor, kinds, labels } from './types'
import { getBoard, getSettings, saveSettings, rotateToken, clearHistory, deleteMonitor, runCheck } from './api'
import { timeLabel, samplesFor, resultLabel } from './stats'
const props = defineProps<{ embedded?: boolean }>()
const route = useRoute()
const token = computed(() => props.embedded ? String(route.query.embed_token || '') : undefined)
const board = ref<Board>(), settings = ref<BoardSettings>(), edit = ref<Monitor>(), detail = ref<Sample>()
const sharing = ref(false), error = ref(''), notice = ref(''), loading = ref(false), saving = ref(false)
const refreshSeconds = ref(15)
const selectedId = ref(''), search = ref('')
const filteredCards = computed(() => (board.value?.cards || []).filter(card => `${card.monitor.name} ${card.monitor.groupName} ${card.monitor.model}`.toLowerCase().includes(search.value.trim().toLowerCase())))
const selectedCard = computed(() => filteredCards.value.find(card => card.monitor.id === selectedId.value) || filteredCards.value[0])
let timer: ReturnType<typeof setTimeout> | undefined, disposed = false
let loadVersion = 0
const embedURL = computed(() => settings.value ? `${window.location.origin}/embed/group-quality?embed_token=${encodeURIComponent(settings.value.token)}` : '')
const embedCode = computed(() => `<iframe src="${embedURL.value}" title="分组质量观测" style="width:100%;height:1000px;border:0" loading="lazy" referrerpolicy="no-referrer"></iframe>`)
function planRefresh() { clearTimeout(timer); if (!disposed && refreshSeconds.value > 0) timer = setTimeout(() => { if (document.visibilityState === 'hidden') planRefresh(); else void load() }, refreshSeconds.value * 1000) }
async function load() {
  if (loading.value || disposed) return
  const version = ++loadVersion
  loading.value = true
  try {
    const result = await getBoard(token.value)
    if (version !== loadVersion || disposed) return
    board.value = result; error.value = ''
  } catch (e) {
    if (version !== loadVersion || disposed) return
    error.value = e instanceof Error ? e.message : '加载失败'
    if (props.embedded) board.value = undefined
  } finally {
    if (version === loadVersion) { loading.value = false; planRefresh() }
  }
}
let restoreTheme: (() => void) | undefined
onBeforeMount(() => {
  if (!props.embedded) return
  const root = document.documentElement
  const wasDark = root.classList.contains('dark')
  restoreTheme = () => { root.classList.toggle('dark', wasDark) }
  // Embedded pages default to light without changing the saved admin theme.
  watch(() => route.query.theme, theme => root.classList.toggle('dark', theme === 'dark'), { immediate: true })
})
onMounted(() => { void load() })
onBeforeUnmount(() => { disposed = true; clearTimeout(timer); restoreTheme?.() })
async function perform(action: () => Promise<unknown>) {
  error.value = ''; notice.value = ''; saving.value = true
  try { await action() } catch (e) { error.value = e instanceof Error ? e.message : '操作失败' } finally { saving.value = false }
}
async function openSharing() { await perform(async () => { settings.value = await getSettings(); sharing.value = true }) }
async function updateSharing(rotate = false) { await perform(async () => { if (!settings.value) return; if (rotate && !window.confirm('重置后旧嵌入链接将立即失效，继续？')) return; settings.value = rotate ? await rotateToken() : await saveSettings(settings.value); notice.value = '嵌入设置已保存'; await load() }) }
async function clearAllHistory() {
  if (saving.value || !window.confirm('确定清空当前工作区所有分组检测项的探活、糖果和鹈鹕历史吗？历史记录和作品无法恢复。分组、模型及自动检测配置会保留，正在运行和后续检测仍会产生新记录。')) return
  await perform(async () => {
    const result = await clearHistory()
    // Discard a refresh started before deletion so stale results cannot reappear.
    ++loadVersion; loading.value = false; clearTimeout(timer)
    if (board.value) board.value = { ...board.value, cards: board.value.cards.map(card => ({ ...card, history: [] })) }
    detail.value = undefined
    notice.value = `已清空 ${result.deleted} 条检测历史，检测配置已保留`
    await load()
  })
}
async function run(id: string, kind: CheckKind) { await perform(async () => { await runCheck(id, kind); notice.value = '已提交后台检测，结果将自动更新'; await load() }) }
async function remove(m: Monitor) { if (!window.confirm(`删除“${m.name}”及其检测历史？`)) return; await perform(async () => { await deleteMonitor(m.id); await load() }) }
async function copied(text: string) { await perform(async () => { await navigator.clipboard.writeText(text); notice.value = '已复制' }) }
function onSaved(monitor: Monitor) { edit.value = undefined; selectedId.value = monitor.id; void load() }
</script>
<template>
  <main class="quality-page" :class="{ embedded }">
    <div class="page-content">
      <header class="page-heading"><div><div class="eyebrow"><Radio :size="14" />SIGNAL DESK / 模型观测工作台</div><h1>{{ embedded ? board?.title || '分组质量观测' : '分组降智检测' }}</h1><p>{{ embedded ? '每一次响应，都是模型状态的一份样本。' : '选择观测对象，追踪响应曲线、答题表现与创作样本。' }}</p></div>
        <div class="page-actions"><button v-if="!embedded" type="button" @click="openSharing"><Code2 :size="16" />嵌入设置</button><button v-if="!embedded" type="button" class="primary" @click="edit = defaultMonitor()"><Plus :size="16" />新建检测</button></div>
      </header>
      <div class="board-toolbar"><span>{{ board?.cards.length || 0 }} 个检测项 <i /> {{ board ? `更新于 ${timeLabel(board.updatedAt)}` : '等待载入' }}</span><div><label>刷新<select v-model.number="refreshSeconds" aria-label="自动刷新间隔" @change="planRefresh"><option :value="0">手动</option><option :value="5">5 秒</option><option :value="15">15 秒</option><option :value="30">30 秒</option><option :value="60">60 秒</option></select></label><button type="button" :disabled="loading" aria-label="刷新结果" @click="load"><RefreshCw :size="15" :class="{ 'animate-spin': loading }" /></button></div></div>
      <p v-if="error" role="alert" class="banner error">{{ error }}</p><p v-if="notice && !embedded" role="status" class="banner notice">{{ notice }}</p>
      <div v-if="!board && loading" class="empty-state">正在读取检测记录…</div>
      <div v-else-if="board && !board.cards.length" class="empty-state"><Radio :size="34" /><h2>从一组观测开始</h2><p>{{ embedded ? '管理员尚未添加检测项' : '选择分组和模型，为三类检测分别设置自动执行间隔' }}</p><button v-if="!embedded" type="button" class="primary" @click="edit = defaultMonitor()">新建分组检测</button></div>
      <div v-else-if="board" class="observation-workspace">
        <aside class="target-rail">
          <div class="rail-heading"><span>观测目录</span><small>{{ board.cards.length.toString().padStart(2, '0') }}</small></div>
          <label class="target-search"><Search :size="13" /><input v-model="search" aria-label="搜索分组或模型" placeholder="搜索分组或模型"></label>
          <nav class="target-list" aria-label="选择观测对象">
            <button v-for="(card, index) in filteredCards" :key="card.monitor.id" type="button" class="target-button" :class="{ selected: selectedCard?.monitor.id === card.monitor.id }" :aria-pressed="selectedCard?.monitor.id === card.monitor.id" @click="selectedId = card.monitor.id">
              <div class="target-number"><span>{{ String(index + 1).padStart(2, '0') }}</span><ArrowUpRight :size="14" /></div>
              <strong>{{ card.monitor.name }}</strong><code>{{ card.monitor.model }}</code>
              <div class="target-signals"><span v-for="kind in kinds" :key="kind" :title="`${labels[kind]}：${samplesFor(card, kind)[0] ? resultLabel(samplesFor(card, kind)[0]!) : '暂无记录'}`"><i :class="samplesFor(card, kind)[0]?.result || 'empty'" />{{ kind === 'health' ? '探活' : kind === 'candy' ? '糖果' : '鹈鹕' }}</span></div>
            </button>
          </nav>
          <p v-if="!filteredCards.length" class="rail-empty">没有匹配的观测对象</p>
          <div class="rail-footnote"><Radio :size="15" /><p>分组与模型独立采样<br>点击目录切换观测对象</p></div>
        </aside>
        <div class="observation-content">
          <template v-if="selectedCard"><MonitorCard :key="selectedCard.monitor.id" :card="selectedCard" :token="token" @edit="edit = selectedCard!.monitor" @run="kind => run(selectedCard!.monitor.id, kind)" @detail="sample => detail = sample" /><button v-if="!embedded" type="button" class="delete-target" :disabled="saving" :aria-label="`删除 ${selectedCard.monitor.name}`" @click="remove(selectedCard.monitor)"><Trash2 :size="12" />移除这个检测项</button></template>
          <div v-else class="selection-empty"><Search :size="28" /><p>换个关键词试试</p></div>
        </div>
      </div>
      <p v-if="board?.cards.length" class="board-footnote">统计来自各项最近 60 次检测。请求异常不计入糖果和鹈鹕的有效判定分母；历史包含手动检测。</p>
      <p class="quality-attribution">糖果与鹈鹕检测由 <a href="https://manxue.ai" target="_blank" rel="noopener noreferrer">manxue.ai</a> 提供</p>
    </div>
    <MonitorForm v-if="edit" :monitor="edit" @close="edit = undefined" @saved="onSaved" />
    <MonitorDetail v-if="detail" :sample="detail" :token="token" @close="detail = undefined" />
    <MonitorDialog v-if="sharing && settings" title="嵌入展示设置" @close="sharing = false">
      <div class="sharing-form"><p class="text-sm text-muted-foreground">开启后，持有链接的人可以查看检测结果与作品；嵌入页面只读。</p><p v-if="error" role="alert" class="banner error">{{ error }}</p><p v-if="notice" role="status" class="banner notice">{{ notice }}</p>
        <div class="history-cleanup"><strong>检测历史</strong><p>清空当前工作区所有分组检测项的探活、糖果和鹈鹕历史及作品，保留分组、模型、自动检测和嵌入配置。正在运行和后续检测会继续产生新记录。</p><button type="button" class="clear-history" :disabled="saving" @click="clearAllHistory"><Trash2 :size="14" />一键清空检测历史</button></div>
        <label>展示标题<input v-model="settings.title" maxlength="120"></label><label class="check-label"><input v-model="settings.enabled" type="checkbox">开启嵌入展示</label><button type="button" class="primary" :disabled="saving" @click="updateSharing()">保存嵌入设置</button>
        <label>嵌入链接<input :value="embedURL" readonly @focus="($event.target as HTMLInputElement).select()"></label><div class="sharing-actions"><button type="button" @click="copied(embedURL)"><Copy :size="14" />复制链接</button><a :href="embedURL" target="_blank" rel="noreferrer"><ExternalLink :size="14" />打开预览</a></div>
        <label>iframe 代码<textarea :value="embedCode" readonly rows="4" /></label><button type="button" @click="copied(embedCode)"><Copy :size="14" />复制嵌入代码</button><button type="button" class="reset-link" :disabled="saving" @click="updateSharing(true)">重置链接（旧链接失效）</button>
      </div>
    </MonitorDialog>
  </main>
</template>
<style scoped>
.history-cleanup{display:flex;flex-direction:column;gap:10px;border-bottom:1px solid hsl(var(--border));padding-bottom:18px;font-size:12px}.history-cleanup p{font-size:11px;line-height:1.8;color:hsl(var(--muted-foreground))}.history-cleanup .clear-history{color:#be4555;border-color:#e3697940;background:#e3697908}
.quality-attribution{text-align:center;font-size:11px;color:hsl(var(--muted-foreground));padding:14px 8px}.quality-attribution a{color:#0f766e;text-decoration:underline;text-underline-offset:3px}.quality-attribution a:hover{color:#0d9488}
.quality-page{padding:28px 30px 40px;min-height:100%;background:radial-gradient(ellipse at 100% 0,#0d948807,transparent 55%)}.quality-page.embedded{min-height:100vh;background:hsl(var(--background));color:hsl(var(--foreground))}.page-content{max-width:1150px;margin:auto}.page-heading{display:flex;justify-content:space-between;align-items:center;gap:20px;margin-bottom:26px}.eyebrow{display:flex;gap:6px;align-items:center;font-size:10px;letter-spacing:1.6px;color:#0f9384;font-weight:650;margin-bottom:8px}h1{font-size:26px;font-weight:650;letter-spacing:-.5px}.page-heading p{font-size:12px;color:hsl(var(--muted-foreground));margin-top:8px}.page-actions{display:flex;gap:10px;flex-shrink:0}button,.sharing-actions a{display:inline-flex;gap:7px;align-items:center;justify-content:center;border:1px solid hsl(var(--border));padding:9px 13px;border-radius:9px;font-size:12px;background:hsl(var(--background));transition:background .15s}button:hover{background:hsl(var(--muted))}button:disabled{opacity:.55;cursor:wait}button.primary{background:#0f766e;color:white;border-color:#0f766e}button.primary:hover{background:#115e59}button:focus-visible,select:focus-visible{outline:2px solid #0d9488;outline-offset:3px}.board-toolbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px;color:hsl(var(--muted-foreground));font-size:11px;gap:12px}.board-toolbar>span{display:flex;gap:9px;align-items:center}.board-toolbar i{width:3px;height:3px;border-radius:50%;background:currentColor}.board-toolbar>div{display:flex;gap:7px;align-items:center}.board-toolbar label{display:flex;gap:6px;align-items:center}.board-toolbar select{background:hsl(var(--background));border:1px solid hsl(var(--border));border-radius:6px;padding:5px}.board-toolbar button{padding:6px}.delete-target{border:0;background:none;padding:7px 2px;float:right;font-size:10px;color:hsl(var(--muted-foreground))}.delete-target:hover{color:#d45768;background:none}.empty-state{min-height:360px;border:1px dashed hsl(var(--border));border-radius:18px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:14px;color:hsl(var(--muted-foreground));text-align:center;padding:20px}.empty-state h2{font-size:20px;color:hsl(var(--foreground))}.empty-state p{font-size:13px}.banner{padding:12px 14px;border-radius:10px;font-size:12px;margin-bottom:16px;overflow-wrap:anywhere}.banner.error{background:#e3697912;color:#be4555;border:1px solid #e3697930}.banner.notice{background:#0d94880b;color:#0f766e;border:1px solid #0d948822}.board-footnote{text-align:center;font-size:10px;color:hsl(var(--muted-foreground));padding:28px 8px}.sharing-form{display:flex;flex-direction:column;gap:16px}.sharing-form label{display:flex;flex-direction:column;gap:7px;font-size:12px}.sharing-form input:not([type=checkbox]),.sharing-form textarea{border:1px solid hsl(var(--border));border-radius:8px;background:hsl(var(--background));padding:10px;font-size:12px;width:100%}.sharing-form .check-label{flex-direction:row;align-items:center}.check-label input{accent-color:#0f766e}.sharing-actions{display:flex;gap:10px}.sharing-form button.reset-link{border:0;color:#c55365}.sharing-form .banner{margin:0}@media(max-width:760px){.quality-page{padding:20px 12px}.page-heading{align-items:flex-start;flex-direction:column}.page-actions{width:100%}.page-actions button{flex:1}.board-toolbar{align-items:flex-start}.board-toolbar>span{flex-direction:column;align-items:flex-start;gap:3px}.board-toolbar i{display:none}h1{font-size:24px}}
.observation-workspace{display:grid;grid-template-columns:218px minmax(0,1fr);border:1px solid hsl(var(--border));border-radius:18px;overflow:hidden;background:hsl(var(--card));box-shadow:0 8px 24px #133d3504}.target-rail{display:flex;flex-direction:column;border-right:1px solid hsl(var(--border));background:hsl(var(--muted)/.3);padding:22px 14px;min-width:0}.rail-heading{display:flex;justify-content:space-between;align-items:center;padding:0 7px;font-size:12px;font-weight:600;letter-spacing:.4px}.rail-heading small{font-family:monospace;font-size:11px;color:#0d9488}.target-search{display:flex;align-items:center;gap:6px;margin:18px 5px 14px;border-bottom:1px solid hsl(var(--border));padding:0 0 9px;color:hsl(var(--muted-foreground))}.target-search input{width:100%;min-width:0;background:transparent;border:0;font-size:10px;outline:none}.target-search:focus-within{border-color:#0d9488}.target-list{display:flex;flex-direction:column;gap:10px}.target-button{display:block;width:100%;padding:14px;text-align:left;border:1px solid transparent;background:transparent;border-radius:10px;transition:background .15s}.target-button:hover{background:hsl(var(--muted))}.target-button.selected{background:hsl(var(--card));border-color:#0d948840;box-shadow:0 3px 12px #0d948808}.target-number{display:flex;align-items:center;justify-content:space-between;color:hsl(var(--muted-foreground));font-size:10px;font-family:monospace;margin-bottom:12px}.target-number svg{opacity:0}.target-button.selected .target-number{color:#0d9488}.target-button.selected .target-number svg{opacity:1}.target-button>strong{display:block;font-size:12px;font-weight:600;line-height:1.6;overflow-wrap:anywhere}.target-button>code{display:block;font-size:9px;color:hsl(var(--muted-foreground));margin-top:4px;overflow-wrap:anywhere}.target-signals{display:flex;gap:10px;margin-top:17px;font-size:9px;color:hsl(var(--muted-foreground))}.target-signals>span{display:flex;align-items:center;gap:4px}.target-signals i{height:4px;width:4px;border-radius:50%}.target-signals .passed{background:#30a995}.target-signals .failed{background:#df6c7e}.target-signals .error{background:#dca457}.target-signals .empty{background:hsl(var(--border))}.rail-footnote{display:flex;align-items:center;gap:8px;color:hsl(var(--muted-foreground));font-size:9px;line-height:1.9;margin-top:auto;padding:36px 7px 0}.rail-empty{font-size:11px;color:hsl(var(--muted-foreground));padding:12px 7px}.observation-content{padding:26px 28px 18px;min-width:0}.delete-target{float:none;display:flex;margin:20px 0 0 auto;font-size:9px}.selection-empty{min-height:480px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:15px;font-size:12px;color:hsl(var(--muted-foreground))}@media(max-width:900px){.observation-workspace{grid-template-columns:180px minmax(0,1fr)}.target-rail{padding:18px 10px}.observation-content{padding:22px 18px}.target-button{padding:11px}.target-signals{gap:7px}}@media(max-width:680px){.observation-workspace{grid-template-columns:1fr}.target-rail{border-right:0;border-bottom:1px solid hsl(var(--border));padding:16px}.rail-heading{padding:0}.target-search{margin:12px 0}.target-list{flex-direction:row;overflow:auto;gap:8px;padding-bottom:5px}.target-button{flex:0 0 170px;width:170px}.target-number{margin-bottom:7px}.target-signals{margin-top:11px}.rail-footnote{display:none}.observation-content{padding:22px 16px}.selection-empty{min-height:180px}}
</style>
