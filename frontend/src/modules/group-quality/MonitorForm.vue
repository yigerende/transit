<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MonitorDialog from './MonitorDialog.vue'
import type { Group, Monitor } from './types'
import { kinds, labels, modeLabels } from './types'
import { getGroups, getModels, saveMonitor } from './api'
const props = defineProps<{ monitor: Monitor }>()
const emit = defineEmits<{ close: []; saved: [monitor: Monitor] }>()
const form = ref<Monitor>(JSON.parse(JSON.stringify(props.monitor)))
const groups = ref<Group[]>([]), models = ref<{ id: string }[]>([])
const error = ref(''), saving = ref(false), discovering = ref(false), loadingGroups = ref(true)
const message = (e: unknown) => e instanceof Error ? e.message : '操作失败，请重试'
async function loadGroups() { loadingGroups.value = true; try { groups.value = await getGroups() } catch (e) { error.value = message(e) } finally { loadingGroups.value = false } }
onMounted(loadGroups)
async function discover() { discovering.value = true; error.value = ''; try { models.value = await getModels(form.value.groupId); if (!models.value.length) error.value = '未获取到模型列表，可以手动填写模型名称' } catch (e) { error.value = message(e) } finally { discovering.value = false } }
async function save() { saving.value = true; error.value = ''; try { const monitor = await saveMonitor(form.value); emit('saved', monitor) } catch (e) { error.value = message(e) } finally { saving.value = false } }
</script>
<template>
  <MonitorDialog :title="form.id ? '检测项设置' : '新建分组检测'" @close="emit('close')">
    <form class="monitor-form" @submit.prevent="save">
      <p v-if="error" role="alert" class="form-error">{{ error }}</p>
      <div class="fields">
        <label>展示名称<input v-model="form.name" required maxlength="120" placeholder="例如：标准组 · 旗舰模型"></label>
        <label>检测分组<select v-model="form.groupId" required :disabled="!!form.id || loadingGroups" @change="models = []"><option value="">{{ loadingGroups ? '正在读取分组…' : '选择分组' }}</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option><option v-if="form.id && !groups.some(g => g.id === form.groupId)" :value="form.groupId">{{ form.groupName }}</option></select></label>
      </div>
      <label>探活 / 糖果检测模型<div class="model-input"><input v-model="form.model" list="group-monitor-models" required maxlength="200" :disabled="!!form.id" placeholder="输入或选择模型"><button type="button" :disabled="!form.groupId || discovering || !!form.id" @click="discover">{{ discovering ? '读取中…' : '获取模型' }}</button></div><datalist id="group-monitor-models"><option v-for="model in models" :key="model.id" :value="model.id" /></datalist></label>
      <p class="help">使用绑定该分组的专用 Key，首次获取模型或检测时自动创建。更换分组或模型请新建检测项。</p>
      <label>API 降智检测地址（可选）<input v-model="form.qualityBaseUrl" type="url" maxlength="2048" placeholder="https://example.com"><span class="help">默认使用工作区地址；需要公开 HTTPS 地址时可单独填写，仍使用原分组 Key。</span></label>
      <label class="switch-label master"><input v-model="form.enabled" type="checkbox">启用本检测项的自动任务<span>关闭后仍可手动检测</span></label>
      <section v-for="kind in kinds" :key="kind" class="check-settings">
        <header><h3>{{ labels[kind] }}</h3><label class="switch-label"><input v-model="form[kind].enabled" type="checkbox">自动检测</label></header>
        <p v-if="kind === 'pelican'" class="help">当前 API 固定检测 gpt-6-astra，作品及历史会标明实际模型。</p>
        <div class="fields">
          <label>间隔（秒）<input v-model.number="form[kind].intervalSeconds" type="number" min="10" max="86400" required></label>
          <label>超时（秒）<input v-model.number="form[kind].timeoutSeconds" type="number" min="5" :max="kind === 'health' ? 300 : 600" required></label>
          <label v-if="kind === 'health'" class="full">探活方式<select v-model="form.health.probeMode"><option v-for="(text, mode) in modeLabels" :key="mode" :value="mode">{{ text }}</option></select></label>
          <template v-else>
            <label>推理强度<select v-model="form[kind].reasoningEffort"><option value="">API 默认</option><option v-for="effort in kind === 'pelican' ? ['low','medium','high'] : ['low','medium','high','xhigh','max','ultra']" :key="effort" :value="effort">{{ effort }}</option></select></label>
            <label>服务档位<select v-model="form[kind].serviceTier"><option value="">默认</option><option value="priority">Priority</option><option value="ultrafast">Ultrafast</option></select></label>
          </template>
        </div>
      </section>
      <p class="help">间隔按每次开始时间计算。同一检测未结束时不会重复执行；仅记录检测结果，不调整渠道优先级或暂停渠道。</p>
      <footer><button type="button" @click="emit('close')">取消</button><button type="submit" class="primary" :disabled="saving">{{ saving ? '保存中…' : '保存设置' }}</button></footer>
    </form>
  </MonitorDialog>
</template>
<style scoped>
.monitor-form{display:flex;flex-direction:column;gap:16px}label{display:flex;flex-direction:column;gap:7px;font-size:12px;font-weight:550}input:not([type=checkbox]),select{background:hsl(var(--background));border:1px solid hsl(var(--border));border-radius:8px;padding:9px 10px;font-size:13px;width:100%;min-width:0}input:focus,select:focus{outline:2px solid #0d9488;outline-offset:1px}input:disabled,select:disabled{opacity:.6}.fields{display:grid;grid-template-columns:1fr 1fr;gap:14px}.full{grid-column:1/-1}.help{font-size:11px;line-height:1.7;color:hsl(var(--muted-foreground));font-weight:400}.model-input{display:flex;gap:8px}.model-input input{flex:1}.model-input button{flex-shrink:0}.switch-label{display:flex;flex-direction:row;align-items:center;gap:8px;cursor:pointer}.switch-label input{accent-color:#0d9488;width:16px;height:16px}.switch-label span{font-size:10px;color:hsl(var(--muted-foreground));font-weight:400}.master{padding:12px 0;border-block:1px solid hsl(var(--border));flex-wrap:wrap}.check-settings{padding:16px;background:hsl(var(--muted)/.45);border:1px solid hsl(var(--border));border-radius:12px}.check-settings header{display:flex;justify-content:space-between;align-items:center;margin-bottom:14px}.check-settings h3{font-size:14px;font-weight:650}.check-settings>.help{margin-bottom:12px}button{padding:8px 14px;border-radius:8px;border:1px solid hsl(var(--border));font-size:12px}button:disabled{opacity:.5}.primary{background:#0f766e;color:white;border-color:#0f766e}footer{display:flex;gap:10px;justify-content:flex-end;margin-top:8px}.form-error{background:#e3697912;color:#c94c5d;border:1px solid #e3697930;border-radius:10px;padding:12px;font-size:12px}@media(max-width:480px){.fields{grid-template-columns:1fr}.full{grid-column:auto}}
</style>
