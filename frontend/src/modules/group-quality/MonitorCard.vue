<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { Activity, Candy, Feather, Settings2, Trash2, Play, LoaderCircle, ArrowUpRight, ScanLine } from 'lucide-vue-next'
import type { Card, CheckKind, Sample } from './types'
import { kinds, labels, modeLabels } from './types'
import { samplesFor, verdictRate, healthAverage, resultLabel, timeLabel, busy } from './stats'
import { getSample } from './api'
import { qualityPreviewDocument } from '../admin/utils/qualityHistory'

const props = defineProps<{ card: Card; token?: string; saving?: boolean }>()
const emit = defineEmits<{ edit: []; remove: []; run: [kind: CheckKind]; detail: [sample: Sample] }>()
const isPublic = computed(() => props.token != null)
const activeKind = ref<CheckKind>('health')
const average = computed(() => healthAverage(props.card))
const qualityKinds = ['candy', 'pelican'] as const
const rates = computed(() => ({ candy: verdictRate(samplesFor(props.card, 'candy')), pelican: verdictRate(samplesFor(props.card, 'pelican')) }))
const history = computed(() => samplesFor(props.card, activeKind.value).slice(0, 60).reverse())
const latest = computed(() => history.value.at(-1))
const errors = computed(() => props.card.history.filter(s => s.result === 'error').length)
const icons = { health: Activity, candy: Candy, pelican: Feather }
const shortTime = (time?: string) => time ? new Date(time).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' }) : '—'

const trend = computed(() => {
  const recent = samplesFor(props.card, 'health').filter(s => s.probeMode === props.card.monitor.health.probeMode).slice(0, 24).reverse()
  const ceiling = Math.max(1000, ...recent.map(s => s.durationMs))
  return recent.map((sample, i) => ({ sample, x: 8 + i * 400 / Math.max(1, recent.length - 1), y: 82 - Math.min(1, sample.durationMs / ceiling) * 64 }))
})
const trendLine = computed(() => trend.value.map(p => `${p.x},${p.y}`).join(' '))
const trendArea = computed(() => trend.value.length ? `8,94 ${trendLine.value} ${trend.value.at(-1)!.x},94` : '')
const artwork = computed(() => samplesFor(props.card, 'pelican').filter(s => s.hasHtml).slice(0, 3))
const chosenArt = ref('')
const featured = computed(() => artwork.value.find(s => s.id === chosenArt.value) || artwork.value[0])
const previews = ref<Record<string, string>>({})
const artworkElement = ref<HTMLElement>()
const artworkVisible = ref(false)
let observer: IntersectionObserver | undefined, previewEpoch = 0
onMounted(() => {
  observer = new IntersectionObserver(entries => {
    if (entries.some(entry => entry.isIntersecting)) { artworkVisible.value = true; observer?.disconnect() }
  }, { rootMargin: '200px' })
  if (artworkElement.value) observer.observe(artworkElement.value)
})
onBeforeUnmount(() => { observer?.disconnect(); previewEpoch++ })
watch(() => `${artworkVisible.value}:${artwork.value.map(s => s.id).join(',')}`, async () => {
  if (!artworkVisible.value) return
  const epoch = ++previewEpoch, next: Record<string, string> = {}
  await Promise.allSettled(artwork.value.map(async sample => {
    if (previews.value[sample.id]) { next[sample.id] = previews.value[sample.id]!; return }
    const result = await getSample(sample.id, props.token)
    if (result.html) next[sample.id] = qualityPreviewDocument(result.html)
  }))
  if (epoch === previewEpoch) previews.value = next
}, { immediate: true })
</script>

<template>
  <article class="monitor-card">
    <header class="station-heading">
      <div><span class="station-caption">当前观测对象</span><h2>{{ card.monitor.name }}</h2><p><span>{{ card.monitor.groupName }}</span><code>{{ card.monitor.model }}</code></p></div>
      <div v-if="!isPublic" class="station-actions"><button type="button" class="setting-button" :disabled="saving" :aria-label="`配置 ${card.monitor.name}`" @click="emit('edit')"><Settings2 :size="15" />设置</button><button type="button" class="setting-button delete-monitor" :disabled="saving" :aria-label="`删除 ${card.monitor.name} 检测项`" @click="emit('remove')"><Trash2 :size="14" />删除</button></div>
    </header>

    <div class="instruments">
      <section class="latency-instrument">
        <div class="instrument-label"><span><Activity :size="14" />{{ card.monitor.health.probeMode === 'first_token' ? '首字响应' : '对话响应' }}</span><small>{{ modeLabels[card.monitor.health.probeMode || ''] }}</small></div>
        <div class="latency-value"><strong>{{ average?.toFixed(2) ?? '—' }}</strong><span>秒<small>近 5 次成功均值</small></span></div>
        <svg class="latency-trend" viewBox="0 0 416 96" preserveAspectRatio="none" role="img" aria-label="最近24次探活延迟趋势">
          <path d="M8 20H408 M8 55H408 M8 90H408" class="chart-grid" />
          <polygon v-if="trend.length" :points="trendArea" class="chart-fill" />
          <polyline v-if="trend.length" :points="trendLine" class="chart-line" />
          <circle v-for="point in trend" :key="point.sample.id" :cx="point.x" :cy="point.y" r="3" :class="point.sample.result === 'passed' ? 'chart-dot' : 'chart-error'"><title>{{ shortTime(point.sample.startedAt) }} · {{ (point.sample.durationMs / 1000).toFixed(2) }}s · {{ resultLabel(point.sample) }}</title></circle>
        </svg>
        <div class="chart-caption"><span>{{ trend.length ? shortTime(trend[0]?.sample.startedAt) : '等待第一轮采样' }}</span><span>{{ trend.length ? '最近 24 次 · ' + shortTime(trend.at(-1)?.sample.startedAt) : '检测完成后绘制响应曲线' }}</span></div>
      </section>
      <div class="quality-instruments">
        <section v-for="kind in qualityKinds" :key="kind" class="quality-instrument" :class="kind">
          <div class="quality-dial"><svg viewBox="0 0 42 42" aria-hidden="true"><circle cx="21" cy="21" r="17" class="dial-track" /><circle cx="21" cy="21" r="17" pathLength="100" class="dial-value" :stroke-dasharray="`${rates[kind].percent || 0} 100`" /></svg><component :is="icons[kind]" :size="19" /></div>
          <div><h3>{{ kind === 'candy' ? '糖果答题' : '鹈鹕创作' }}<span>API</span></h3><p>{{ rates[kind].passed }} / {{ rates[kind].total }} 次有效判定<span v-if="rates[kind].errors"> · {{ rates[kind].errors }} 次异常</span></p></div>
          <strong>{{ rates[kind].percent ?? '—' }}<small v-if="rates[kind].percent != null">%</small></strong>
        </section>
        <div class="sampling-note"><span class="signal-dot" />{{ card.monitor.enabled && kinds.some(k => card.monitor[k].enabled) ? '自动观测已开启' : '当前为手动观测' }}<span>{{ errors }} 次请求异常</span></div>
      </div>
    </div>

    <section class="record-panel">
      <header><div class="section-title"><span class="section-number">01</span><h3>检测时间线</h3></div><span class="subtle">最近 60 次</span></header>
      <div class="record-controls"><div class="check-tabs" role="tablist" aria-label="检测记录类型"><button v-for="kind in kinds" :id="`check-tab-${kind}`" :key="kind" type="button" role="tab" :aria-selected="activeKind === kind" aria-controls="check-record-panel" :class="{ selected: activeKind === kind }" @click="activeKind = kind"><component :is="icons[kind]" :size="14" />{{ kind === 'health' ? '探活' : kind === 'candy' ? '糖果' : '鹈鹕' }}</button></div><button v-if="!isPublic" type="button" class="run-button" :disabled="busy(card, activeKind)" :aria-label="`立即${labels[activeKind]} ${card.monitor.name}`" @click="emit('run', activeKind)"><LoaderCircle v-if="busy(card, activeKind)" :size="13" class="animate-spin" /><Play v-else :size="13" />{{ busy(card, activeKind) ? '检测中' : '立即检测' }}</button></div>
      <div id="check-record-panel" role="tabpanel" :aria-labelledby="`check-tab-${activeKind}`">
        <div class="record-context"><span>{{ labels[activeKind] }}<template v-if="activeKind === 'pelican'"> · gpt-6-astra</template></span><span>{{ card.monitor.enabled && card.monitor[activeKind].enabled ? `每 ${card.monitor[activeKind].intervalSeconds} 秒自动执行` : '自动检测关闭' }}</span></div>
        <div class="timeline" :class="activeKind" aria-label="检测历史，从左到右、从上到下由旧到新">
          <span v-for="empty in Math.max(0, 60 - history.length)" :key="`empty-${empty}`" class="sample empty" />
          <button v-for="sample in history" :key="sample.id" type="button" class="sample" :class="sample.result" :aria-label="`${labels[activeKind]} ${timeLabel(sample.startedAt)} ${resultLabel(sample)}`" :title="`${timeLabel(sample.startedAt)} · ${resultLabel(sample)} · ${(sample.durationMs / 1000).toFixed(2)}s`" @click="emit('detail', sample)" />
        </div>
        <div class="history-footer"><span>{{ latest ? `最近检测 ${timeLabel(latest.startedAt)}` : '暂无记录，自动或手动检测后会显示在这里' }}</span><span class="legend"><i class="passed" />通过<i class="failed" />未通过<i class="error" />异常</span></div>
      </div>
    </section>

    <section ref="artworkElement" class="artwork-section">
      <header><div class="section-title"><span class="section-number">02</span><h3>创作样本室</h3></div><span class="subtle">鹈鹕 API · gpt-6-astra</span></header>
      <div v-if="featured" class="artwork-layout">
        <button type="button" class="featured-art" :aria-label="`查看 ${timeLabel(featured.startedAt)} 鹈鹕作品`" @click="emit('detail', featured)"><div class="preview"><iframe v-if="previews[featured.id]" :srcdoc="previews[featured.id]" sandbox="allow-scripts" referrerpolicy="no-referrer" tabindex="-1" title="鹈鹕作品预览" /><span v-else>点击加载作品</span></div><div class="artwork-caption"><span>{{ timeLabel(featured.startedAt) }}</span><span>查看完整作品<ArrowUpRight :size="14" /></span></div></button>
        <div class="artwork-index" aria-label="近期作品选择"><button v-for="(sample, index) in artwork" :key="sample.id" type="button" :class="{ selected: featured.id === sample.id }" :aria-pressed="featured.id === sample.id" @click="chosenArt = sample.id"><span class="mini-preview"><iframe v-if="previews[sample.id]" :srcdoc="previews[sample.id]" sandbox="allow-scripts" referrerpolicy="no-referrer" tabindex="-1" :title="`作品缩略图 ${index + 1}`" /><span v-else>0{{ index + 1 }}</span></span><div><strong>{{ index === 0 ? '最新样本' : '历史样本' }}</strong><small>{{ shortTime(sample.startedAt) }} · {{ resultLabel(sample) }}</small></div></button><p>保留近期 3 份创作<br>点击大画布查看详情</p></div>
      </div>
      <div v-else class="no-artwork"><div class="empty-orbit"><ScanLine :size="34" /></div><div><h4>等待第一份创作样本</h4><p>完成 API 鹈鹕检测后，作品会在这里归档。</p></div><button v-if="!isPublic" type="button" class="run-button" :disabled="busy(card, 'pelican')" @click="emit('run', 'pelican')">{{ busy(card, 'pelican') ? '创作中…' : '启动鹈鹕检测' }}<ArrowUpRight :size="14" /></button></div>
    </section>
  </article>
</template>

<style scoped>
.station-actions{display:flex;flex-wrap:wrap;gap:8px;flex-shrink:0}.station-actions .delete-monitor{color:#be4555;border-color:#e3697940}.station-actions button:disabled{opacity:.5;cursor:wait}@media(max-width:600px){.station-heading{flex-wrap:wrap}}
.monitor-card{min-width:0}.station-heading{display:flex;justify-content:space-between;align-items:center;gap:16px;margin-bottom:24px}.station-caption{font-size:10px;letter-spacing:1px;color:hsl(var(--muted-foreground))}.station-heading h2{font-size:23px;font-weight:650;letter-spacing:-.7px;margin:5px 0 9px;overflow-wrap:anywhere}.station-heading p{display:flex;flex-wrap:wrap;align-items:center;gap:8px;font-size:11px;color:hsl(var(--muted-foreground))}.station-heading p>span{padding:3px 7px;background:hsl(var(--muted));border-radius:5px}.station-heading code{font-size:11px}.setting-button,.run-button{display:inline-flex;align-items:center;justify-content:center;gap:6px;padding:8px 11px;font-size:11px;border:1px solid hsl(var(--border));border-radius:7px;background:hsl(var(--card));white-space:nowrap}.run-button{color:#0d9488}.setting-button:hover,.run-button:hover{border-color:#0d9488}.run-button:disabled{opacity:.5;cursor:wait}.instruments{display:grid;grid-template-columns:1.1fr 1fr;gap:20px;margin-bottom:28px}.latency-instrument{border:1px solid #0d94882c;border-radius:14px;padding:18px;background:linear-gradient(145deg,#0d94880c,transparent)}.instrument-label{display:flex;justify-content:space-between;align-items:center;font-size:11px;gap:8px}.instrument-label>span{display:flex;align-items:center;gap:6px;color:#0d9488;font-weight:600}.instrument-label small{color:hsl(var(--muted-foreground));font-size:10px}.latency-value{display:flex;align-items:center;gap:13px;margin:12px 0 0}.latency-value strong{font-size:46px;font-weight:550;line-height:1.1;letter-spacing:-2px;color:#0d9488;font-variant-numeric:tabular-nums}.latency-value>span{font-size:12px;color:hsl(var(--muted-foreground))}.latency-value small{display:block;font-size:9px;margin-top:5px}.latency-trend{display:block;width:100%;height:76px;margin-top:12px;overflow:visible}.chart-grid{fill:none;stroke:hsl(var(--border));stroke-dasharray:3 6;stroke-width:.7}.chart-fill{fill:#0d94880c}.chart-line{fill:none;stroke:#0d9488;stroke-width:1.7;vector-effect:non-scaling-stroke}.chart-dot{fill:#0d9488}.chart-error{fill:#d59641;stroke:hsl(var(--card));stroke-width:1}.chart-caption{display:flex;justify-content:space-between;font-size:9px;color:hsl(var(--muted-foreground));margin-top:4px}.quality-instruments{display:flex;flex-direction:column;gap:11px}.quality-instrument{display:flex;align-items:center;gap:12px;border:1px solid hsl(var(--border));border-radius:12px;padding:16px 14px;background:hsl(var(--card));flex:1;min-width:0}.quality-dial{width:44px;height:44px;position:relative;display:grid;place-items:center;flex-shrink:0;color:#d18d47}.quality-dial>svg:first-child{position:absolute;inset:0;width:100%;height:100%;transform:rotate(-90deg)}.dial-track,.dial-value{fill:none;stroke-width:2.5}.dial-track{stroke:hsl(var(--border))}.dial-value{stroke:currentColor;stroke-linecap:round}.pelican .quality-dial{color:#647fb8}.quality-instrument h3{font-size:12px;font-weight:600}.quality-instrument h3 span{font-size:8px;font-weight:400;border:1px solid hsl(var(--border));padding:1px 3px;border-radius:3px;margin-left:6px;color:hsl(var(--muted-foreground))}.quality-instrument p{font-size:9px;color:hsl(var(--muted-foreground));margin-top:6px;line-height:1.7}.quality-instrument>strong{margin-left:auto;font-size:23px;font-weight:550;letter-spacing:-.8px;font-variant-numeric:tabular-nums}.quality-instrument>strong small{font-size:10px;margin-left:2px;font-weight:400}.sampling-note{display:flex;align-items:center;gap:5px;font-size:9px;color:hsl(var(--muted-foreground));padding:0 3px}.sampling-note>span:last-child{margin-left:auto}.signal-dot{height:5px;width:5px;border-radius:50%;background:#0d9488}.record-panel,.artwork-section{border-top:1px solid hsl(var(--border));padding-top:22px;margin-top:24px}.record-panel>header,.artwork-section>header{display:flex;justify-content:space-between;align-items:center;gap:10px;margin-bottom:18px}.section-title{display:flex;align-items:center;gap:9px}.section-title h3{font-size:13px;font-weight:650}.section-number{font-family:monospace;color:#0d9488;font-size:10px;letter-spacing:1px}.subtle{font-size:9px;color:hsl(var(--muted-foreground))}.record-controls{display:flex;justify-content:space-between;align-items:center;gap:10px}.check-tabs{display:flex;gap:2px;padding:3px;border-radius:8px;background:hsl(var(--muted))}.check-tabs button{display:flex;align-items:center;gap:6px;font-size:11px;color:hsl(var(--muted-foreground));padding:7px 13px;border-radius:6px}.check-tabs button.selected{background:hsl(var(--card));color:#0d9488;box-shadow:0 1px 3px #0000000b}.record-context{display:flex;align-items:center;justify-content:space-between;gap:8px;font-size:10px;color:hsl(var(--muted-foreground));margin:18px 0 11px}.timeline{display:grid;grid-template-columns:repeat(30,minmax(0,1fr));gap:5px}.sample{height:15px;width:100%;border-radius:3px;transition:transform .15s}.candy .sample{height:12px;border-radius:8px}.pelican .sample{border-radius:1px 6px 1px 6px}.sample:hover:not(.empty),.sample:focus-visible{transform:translateY(-2px);outline:2px solid hsl(var(--foreground));outline-offset:1px}.empty{background:hsl(var(--muted-foreground)/.12)}.passed{background:#30a995}.failed{background:#df6c7e}.error{background:#dca457}.history-footer{display:flex;justify-content:space-between;gap:8px;color:hsl(var(--muted-foreground));font-size:9px;margin-top:12px}.legend{display:flex;align-items:center;gap:4px;flex-shrink:0}.legend i{width:5px;height:5px;border-radius:50%;margin-left:5px}.artwork-layout{display:grid;grid-template-columns:minmax(0,1fr) 150px;gap:18px}.featured-art{border:1px solid hsl(var(--border));border-radius:10px;overflow:hidden;text-align:left;min-width:0}.featured-art:hover{border-color:#0d9488}.preview{aspect-ratio:4/3;min-height:300px;overflow:hidden;background:#ecf0ed;position:relative;display:grid;place-items:center}.preview>span{font-size:12px;color:#62736d}.preview iframe{width:200%;height:200%;transform:scale(.5);transform-origin:top left;border:0;pointer-events:none;position:absolute;top:0;left:0;background:white}.artwork-caption{display:flex;align-items:center;justify-content:space-between;gap:6px;padding:11px 12px;font-size:9px;color:hsl(var(--muted-foreground));background:hsl(var(--card))}.artwork-caption>span:last-child{display:flex;align-items:center;gap:3px}.artwork-index{display:flex;flex-direction:column;gap:9px}.artwork-index button{display:flex;align-items:center;gap:8px;padding:13px 10px;border:1px solid transparent;border-radius:8px;text-align:left;background:hsl(var(--muted)/.5)}.artwork-index button.selected{border-color:#0d948840;background:#0d94880b}.art-index{font-family:monospace;color:#0d9488;font-size:11px}.artwork-index strong{display:block;font-size:11px;font-weight:550}.artwork-index small{font-size:9px;color:hsl(var(--muted-foreground));display:block;margin-top:4px}.artwork-index button>svg{margin-left:auto;color:hsl(var(--muted-foreground))}.artwork-index>p{margin-top:auto;font-size:9px;line-height:1.8;color:hsl(var(--muted-foreground));padding:0 5px}.no-artwork{display:flex;align-items:center;gap:20px;min-height:170px;padding:26px;background:radial-gradient(hsl(var(--border)) .7px,transparent .7px);background-size:12px 12px;border:1px solid hsl(var(--border));border-radius:12px}.empty-orbit{border:1px solid #0d948830;border-radius:50%;padding:20px;color:#0d9488;outline:1px dashed #0d948825;outline-offset:7px}.no-artwork h4{font-size:13px;font-weight:550}.no-artwork p{font-size:10px;color:hsl(var(--muted-foreground));margin-top:7px}.no-artwork>.run-button{margin-left:auto}button:focus-visible{outline:2px solid #0d9488;outline-offset:3px}@media(max-width:1100px){.instruments{grid-template-columns:1fr}.quality-instruments{display:grid;grid-template-columns:1fr 1fr}.sampling-note{grid-column:1/-1}.quality-instrument{padding:12px 10px;gap:8px}.quality-instrument>strong{font-size:20px}.quality-dial{width:34px;height:34px}.quality-instrument p span{display:none}.artwork-layout{grid-template-columns:minmax(0,1fr) 125px}.no-artwork{flex-wrap:wrap}.no-artwork>.run-button{margin-left:0}}@media(max-width:600px){.station-heading h2{font-size:20px}.quality-instruments{grid-template-columns:1fr}.quality-instrument p span{display:inline}.instruments{gap:13px}.artwork-layout{grid-template-columns:1fr}.artwork-index{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:6px}.artwork-index>p,.artwork-index button>svg{display:none}.artwork-index button{padding:10px 7px;gap:6px}.art-index{display:none}.timeline{grid-template-columns:repeat(20,minmax(0,1fr));gap:4px}.check-tabs button{padding:7px 10px}.history-footer{flex-wrap:wrap}.no-artwork{padding:25px;flex-direction:column;text-align:center}.record-controls{gap:5px}.run-button{padding:8px}.artwork-caption{font-size:8px}.station-heading{align-items:flex-start}}
.mini-preview{display:grid;place-items:center;width:32px;height:38px;background:#edf3ee;border-radius:4px;position:relative;overflow:hidden;flex-shrink:0;font-family:monospace;color:#0d9488;font-size:11px}.mini-preview iframe{position:absolute;left:0;top:0;width:400px;height:475px;transform:scale(.08);transform-origin:top left;pointer-events:none;background:white;border:0}@media(max-width:600px){.mini-preview{display:none}}
</style>
