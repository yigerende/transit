<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { cachedThumbnail, queueThumbnail, rememberThumbnail, staticThumbnailDocument, thumbnailCaptureDocument, thumbnailMaxBytes } from './artworkSnapshot'

const props = defineProps<{ html?: string; title: string; index: number }>()
const frame = ref<HTMLIFrameElement>()
const snapshot = ref(''), captureDocument = ref(''), desktop = ref(false)
const media = window.matchMedia('(min-width: 601px)')
const updateMedia = () => { desktop.value = media.matches }
onMounted(() => { updateMedia(); media.addEventListener('change', updateMedia) })
onBeforeUnmount(() => media.removeEventListener('change', updateMedia))

watch(() => [props.html, desktop.value] as const, ([html, visible], _, onCleanup) => {
  let cancelled = false, cancelCapture: (() => void) | undefined
  snapshot.value = ''; captureDocument.value = ''
  onCleanup(() => { cancelled = true; cancelCapture?.() })
  if (!html || !visible) return
  const cached = cachedThumbnail(html)
  if (cached) { snapshot.value = cached; return }
  void queueThumbnail(async () => {
    if (cancelled) return
    const reused = cachedThumbnail(html)
    if (reused) { snapshot.value = reused; return }
    await new Promise<void>(resolve => {
      const token = crypto.randomUUID()
      let timer: ReturnType<typeof setTimeout>, finished = false
      const finish = (value = '') => {
        if (finished) return
        finished = true; clearTimeout(timer); window.removeEventListener('message', receive)
        captureDocument.value = ''
        try {
          if (!cancelled && value) {
            const result = staticThumbnailDocument(value)
            rememberThumbnail(html, result); snapshot.value = result
          }
        } finally { resolve() }
      }
      const receive = (event: MessageEvent) => {
        if (event.source !== frame.value?.contentWindow || event.data?.type !== 'transit-artwork-thumbnail' || event.data?.token !== token) return
        if (typeof event.data.html !== 'string' || event.data.html.length > thumbnailMaxBytes) return
        finish(event.data.html)
      }
      cancelCapture = () => finish()
      window.addEventListener('message', receive)
      // Broken artwork must neither keep scripts running nor block later previews.
      timer = setTimeout(() => finish(), 2500)
      captureDocument.value = thumbnailCaptureDocument(html, token)
    })
  })
}, { immediate: true })
</script>

<template>
  <iframe v-if="desktop && snapshot" :srcdoc="snapshot" sandbox="" referrerpolicy="no-referrer" tabindex="-1" :title="title" data-artwork-thumbnail="static" />
  <iframe v-else-if="desktop && captureDocument" ref="frame" :srcdoc="captureDocument" sandbox="allow-scripts" referrerpolicy="no-referrer" tabindex="-1" :title="title" data-artwork-thumbnail="capturing" />
  <span v-else>{{ String(index + 1).padStart(2, '0') }}</span>
</template>
