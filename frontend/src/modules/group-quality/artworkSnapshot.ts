import { qualityPreviewDocument } from '../admin/utils/qualityHistory'

export const thumbnailMaxBytes = 750_000
const cache = new Map<string, string>()
let queue = Promise.resolve()

export function cachedThumbnail(source: string): string | undefined {
  const result = cache.get(source)
  if (result) { cache.delete(source); cache.set(source, result) }
  return result
}

export function rememberThumbnail(source: string, snapshot: string) {
  cache.delete(source); cache.set(source, snapshot)
  let size = [...cache].reduce((total, [key, value]) => total + key.length + value.length, 0)
  while (cache.size > 12 || size > 3_000_000) {
    const [key, value] = cache.entries().next().value!
    size -= key.length + value.length; cache.delete(key)
  }
}

// Capture only one thumbnail at a time. The queue also releases on cancellation.
export function queueThumbnail(capture: () => Promise<void>) {
  queue = queue.then(capture, capture).catch(() => {})
  return queue
}

export function thumbnailCaptureDocument(source: string, token: string): string {
  const document = new DOMParser().parseFromString(qualityPreviewDocument(source), 'text/html')
  const script = document.createElement('script')
  script.textContent = `(() => {
    const send = parent.postMessage.bind(parent);
    addEventListener('load', () => setTimeout(() => {
      try {
        const root = document.documentElement.cloneNode(true);
        const originals = [document.documentElement, ...document.documentElement.querySelectorAll('*')];
        const copies = [root, ...root.querySelectorAll('*')];
        const pairs = new Map(originals.map((element, index) => [element, copies[index]]));
        // Preserve the current CSS/Web Animation pose before disabling animation.
        for (const animation of document.getAnimations()) {
          const effect = animation.effect, target = effect && effect.target, copy = pairs.get(target);
          if (!copy || !copy.style || !effect.getKeyframes) continue;
          const computed = getComputedStyle(target);
          for (const keyframe of effect.getKeyframes()) for (const property of Object.keys(keyframe)) {
            if (['offset', 'computedOffset', 'easing', 'composite'].includes(property)) continue;
            const name = property.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase());
            copy.style.setProperty(name, computed.getPropertyValue(name), 'important');
          }
        }
        // Canvas pixels are not included in outerHTML. Save them before the live
        // sandbox is destroyed, so script-drawn artwork keeps a useful preview.
        for (const canvas of document.querySelectorAll('canvas')) {
          const copy = pairs.get(canvas), image = document.createElement('img');
          for (const attribute of canvas.attributes) image.setAttribute(attribute.name, attribute.value);
          image.src = canvas.toDataURL('image/png');
          image.width = canvas.width; image.height = canvas.height;
          const computed = getComputedStyle(canvas);
          image.style.width = computed.width; image.style.height = computed.height;
          copy.replaceWith(image);
        }
        const html = '<!doctype html>\\n' + root.outerHTML;
        send({ type: 'transit-artwork-thumbnail', token: ${JSON.stringify(token)}, html: html.length <= ${thumbnailMaxBytes} ? html : '' }, '*');
      } catch { send({ type: 'transit-artwork-thumbnail', token: ${JSON.stringify(token)}, html: '' }, '*'); }
    }, 350), { once: true });
  })();`
  document.head.append(script)
  return '<!doctype html>\n' + document.documentElement.outerHTML
}

export function staticThumbnailDocument(snapshot: string): string {
  const document = new DOMParser().parseFromString(qualityPreviewDocument(snapshot), 'text/html')
  document.querySelectorAll('script, animate, animateMotion, animateTransform, set').forEach(node => node.remove())
  document.querySelectorAll('*').forEach(node => {
    for (const attribute of [...node.attributes]) if (attribute.name.toLowerCase().startsWith('on')) node.removeAttribute(attribute.name)
  })
  const policy = document.querySelector('meta[http-equiv="Content-Security-Policy"]')!
  policy.setAttribute('content', policy.getAttribute('content')!.replace("script-src 'unsafe-inline'", "script-src 'none'"))
  const style = document.createElement('style')
  style.textContent = '*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}'
  document.head.append(style)
  return '<!doctype html>\n' + document.documentElement.outerHTML
}
