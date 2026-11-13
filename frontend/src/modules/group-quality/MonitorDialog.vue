<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { X } from 'lucide-vue-next'
defineProps<{ title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement>()
onMounted(() => dialog.value?.showModal())
</script>
<template>
  <dialog ref="dialog" class="monitor-dialog" :class="{ wide }" aria-labelledby="monitor-dialog-title" @cancel.prevent="emit('close')" @click="e => { if (e.target === dialog) emit('close') }">
    <header><h2 id="monitor-dialog-title">{{ title }}</h2><button type="button" aria-label="关闭" @click="emit('close')"><X :size="20" /></button></header>
    <div class="dialog-body"><slot /></div>
  </dialog>
</template>
<style scoped>
.monitor-dialog{padding:0;width:min(700px,calc(100vw - 32px));max-height:90dvh;border:1px solid hsl(var(--border));border-radius:18px;background:hsl(var(--background));color:hsl(var(--foreground));box-shadow:0 24px 80px #0004}.monitor-dialog.wide{width:min(1100px,calc(100vw - 32px))}.monitor-dialog::backdrop{background:#10242580;backdrop-filter:blur(4px)}header{display:flex;justify-content:space-between;align-items:center;padding:20px 24px;border-bottom:1px solid hsl(var(--border));position:sticky;top:0;background:hsl(var(--background));z-index:1}h2{font-size:18px;font-weight:650}header button{padding:6px;border-radius:8px}header button:hover{background:hsl(var(--muted))}.dialog-body{padding:24px}
</style>
