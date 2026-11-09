<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSmugglerStore } from '../stores/smuggler'
import { useAppStore } from '../stores/app'

const smuggler = useSmugglerStore()
const app = useAppStore()
const viewport = ref<HTMLElement | null>(null)
const measure = ref<HTMLElement | null>(null)
const distance = ref(0)
const duration = ref(18)
let observer: ResizeObserver | undefined

function updateMarquee() {
  const width = measure.value?.scrollWidth || 0
  distance.value = width > (viewport.value?.clientWidth || 0) ? width + 32 : 0
  duration.value = Math.max(12, distance.value / 38)
}
watch(() => smuggler.marqueeText, async () => {
  await nextTick()
  observer?.disconnect()
  if (viewport.value) observer?.observe(viewport.value)
  updateMarquee()
})
onMounted(() => {
  observer = new ResizeObserver(updateMarquee)
  if (viewport.value) observer.observe(viewport.value)
  updateMarquee()
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <button v-if="smuggler.marqueeText" type="button" class="smuggler-banner" title="查看走私商人" @click="app.setActiveTab('tradeSmuggler')">
    <span class="banner-label">走私</span>
    <span ref="viewport" class="marquee-viewport">
      <span ref="measure" class="marquee-measure" aria-hidden="true">{{ smuggler.marqueeText }}</span>
      <span class="marquee-track" :class="{ scrolling: distance > 0 }" :style="{ '--marquee-distance': `${distance}px`, '--marquee-duration': `${duration}s` }">
        <span>{{ smuggler.marqueeText }}</span>
        <span v-if="distance > 0" aria-hidden="true">{{ smuggler.marqueeText }}</span>
      </span>
    </span>
  </button>
</template>

<style scoped>
.smuggler-banner { display: flex; align-items: center; gap: 8px; flex-shrink: 0; width: 100%; padding: 6px 10px; border: 0; border-bottom: 1px solid rgba(255, 183, 77, 0.14); background: rgba(255, 183, 77, 0.06); color: #ccc; font: inherit; text-align: left; cursor: pointer; }
.smuggler-banner:hover { background: rgba(255, 183, 77, 0.1); }
.smuggler-banner:focus-visible { outline: 1px solid #ffb74d; outline-offset: -1px; }
.banner-label { flex-shrink: 0; padding: 1px 5px; border-radius: 3px; color: #ffb74d; background: rgba(255, 183, 77, 0.1); font-size: 10px; }
.marquee-viewport { position: relative; flex: 1; min-width: 0; overflow: hidden; font-size: 11px; }
.marquee-measure { position: absolute; visibility: hidden; white-space: nowrap; }
.marquee-track { display: flex; width: max-content; gap: 32px; white-space: nowrap; }
.marquee-track.scrolling { animation: smuggler-scroll var(--marquee-duration) linear infinite; }
.smuggler-banner:hover .marquee-track { animation-play-state: paused; }
@keyframes smuggler-scroll { to { transform: translateX(calc(-1 * var(--marquee-distance))); } }
@media (prefers-reduced-motion: reduce) { .marquee-track.scrolling { animation: none; } }
</style>
