<script setup lang="ts">
import { computed, nextTick, ref, useId } from 'vue'
import { onClickOutside, useElementBounding, useEventListener, useWindowSize } from '@vueuse/core'
import { ChevronDown } from 'lucide-vue-next'

defineProps<{ label: string; title?: string }>()
const open = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLDivElement | null>(null)
const id = useId()
const { top, bottom, right, update } = useElementBounding(trigger)
const { height: panelHeight } = useElementBounding(panel)
const { width, height } = useWindowSize()
const position = computed(() => ({
  left: `${Math.max(8, Math.min(right.value - 256, width.value - 264))}px`,
  top: `${Math.max(8, Math.min(height.value - panelHeight.value - 8, bottom.value + 4 + panelHeight.value <= height.value - 8 ? bottom.value + 4 : top.value - panelHeight.value - 4))}px`,
  maxHeight: `${Math.max(120, height.value - 16)}px`,
}))
const close = (restoreFocus = false) => {
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}
const show = async () => {
  update()
  open.value = true
  await nextTick()
  panel.value?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
}
onClickOutside(panel, () => close(), { ignore: [trigger] })
useEventListener(document, 'focusin', (event) => {
  if (open.value && event.target instanceof Node && !panel.value?.contains(event.target) && !trigger.value?.contains(event.target)) close()
})
const onKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    close(true)
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const buttons = Array.from(panel.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])
    if (!buttons.length) return
    const current = buttons.indexOf(document.activeElement as HTMLButtonElement)
    buttons[(current + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length]?.focus()
  }
}
</script>

<template>
  <button ref="trigger" type="button" :aria-label="label" :title="title" :aria-expanded="open" :aria-controls="open ? id : undefined" aria-haspopup="dialog" class="inline-flex h-7 shrink-0 items-center gap-1 rounded border border-border/60 px-2 text-xs text-muted-foreground hover:bg-primary/10 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary" @click.stop="open ? close() : show()" @keydown.down.prevent="show()" @keydown.esc.stop.prevent="close(true)">
    <slot name="trigger">{{ label }}</slot>
    <ChevronDown class="h-3 w-3 transition-transform" :class="{ 'rotate-180': open }" />
  </button>
  <Teleport to="body">
    <div v-if="open" :id="id" ref="panel" role="dialog" :aria-label="label" :style="position" class="fixed z-50 w-64 max-w-[calc(100vw-1rem)] overflow-y-auto rounded-lg border border-border bg-card p-1.5 text-foreground shadow-lg" @keydown="onKeydown">
      <slot :close="() => close(true)" />
    </div>
  </Teleport>
</template>
