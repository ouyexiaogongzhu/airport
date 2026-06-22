<template>
  <div class="qr-wrapper">
    <canvas ref="canvas"></canvas>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import QRCode from 'qrcode'

const props = defineProps<{ url: string }>()
const canvas = ref<HTMLCanvasElement | null>(null)

async function render() {
  if (!canvas.value || !props.url) return
  try {
    await QRCode.toCanvas(canvas.value, props.url, {
      width: 200,
      margin: 2,
      color: { dark: '#e0e0e0', light: '#1a1a2e' },
    })
  } catch (e) {
    console.error('QR render failed:', e)
  }
}

onMounted(render)
watch(() => props.url, render)
</script>

<style scoped>
.qr-wrapper {
  display: flex;
  justify-content: center;
  padding: 0.5rem 0;
}
canvas {
  border-radius: 8px;
}
</style>
