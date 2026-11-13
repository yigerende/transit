<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import MonitorDialog from './MonitorDialog.vue'
import { getSample } from './api'
import type { Sample } from './types'
import { labels, modeLabels } from './types'
import { resultLabel, timeLabel } from './stats'
import { qualityEstimatedCostUSD, qualityFormatCost, qualityPreviewDocument } from '../admin/utils/qualityHistory'
const props = defineProps<{ sample: Sample; token?: string }>()
const emit = defineEmits<{ close: [] }>()
const { t, te } = useI18n()
const detail = ref(props.sample), loading = ref(true), error = ref('')
const document = computed(() => detail.value.html ? qualityPreviewDocument(detail.value.html) : '')
onMounted(async () => { try { detail.value = await getSample(props.sample.id, props.token) } catch (e) { error.value = e instanceof Error ? e.message : '详情加载失败' } finally { loading.value = false } })
</script>
<template>
  <MonitorDialog :title="`${labels[detail.kind]} · ${resultLabel(detail)}`" wide @close="emit('close')">
    <p v-if="error" role="alert" class="text-red-600 mb-4">{{ error }}</p>
    <div class="detail-metrics">
      <div><span>开始时间</span><strong>{{ timeLabel(detail.startedAt) }}</strong></div><div><span>结束时间</span><strong>{{ timeLabel(detail.createdAt) }}</strong></div>
      <div><span>实际检测模型</span><strong>{{ detail.model }}</strong></div><div><span>耗时{{ detail.probeMode === 'first_token' ? '（首字）' : '' }}</span><strong>{{ (detail.durationMs / 1000).toFixed(2) }} s</strong></div>
      <template v-if="detail.kind !== 'health'"><div><span>输入 / 输出 Token</span><strong>{{ detail.inputTokens ?? '—' }} / {{ detail.outputTokens ?? '—' }}</strong></div><div><span>参考预估费用</span><strong>{{ qualityFormatCost(qualityEstimatedCostUSD(detail)) }}</strong></div></template>
      <div v-else><span>探活方式</span><strong>{{ modeLabels[detail.probeMode || ''] || '—' }}</strong></div><div><span>触发方式</span><strong>{{ detail.manual ? '手动' : '自动' }}</strong></div>
    </div>
    <p v-if="detail.kind !== 'health'" class="text-xs text-muted-foreground mb-5">费用按满血网页参考单价估算，不是实际账单；接口未提供用量时显示 —。</p>
    <p v-if="loading" role="status" class="py-8 text-center">正在加载检测内容…</p>
    <template v-else>
      <p v-if="detail.errorKey" class="result-note">{{ te(detail.errorKey) ? t(detail.errorKey) : detail.errorKey }}<br v-if="detail.errorDetail">{{ detail.errorDetail }}</p>
      <p v-if="detail.report" class="result-note">{{ detail.report }}</p>
      <pre v-if="detail.answer" class="answer">{{ detail.answer }}</pre>
      <iframe v-if="document" :srcdoc="document" sandbox="allow-scripts" referrerpolicy="no-referrer" title="鹈鹕检测作品" class="art-detail" />
      <p v-else-if="detail.kind === 'pelican'" class="text-muted-foreground py-8 text-center">{{ detail.htmlTooLarge ? '作品内容超出保存上限' : '本次检测没有可展示的作品' }}</p>
    </template>
  </MonitorDialog>
</template>
<style scoped>
.detail-metrics{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px;margin-bottom:20px}.detail-metrics span{display:block;font-size:12px;color:hsl(var(--muted-foreground));margin-bottom:5px}.detail-metrics strong{font-size:14px;font-weight:600;overflow-wrap:anywhere}.result-note,.answer{white-space:pre-wrap;overflow-wrap:anywhere;padding:16px;background:hsl(var(--muted));border-radius:12px;font-size:13px;margin-bottom:16px}.art-detail{width:100%;height:65vh;border:1px solid hsl(var(--border));border-radius:12px;background:white}@media(max-width:500px){.detail-metrics{grid-template-columns:1fr}}
</style>
