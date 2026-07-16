<template>
  <div class="qr-wrapper">
    <canvas v-show="!errorMsg" ref="canvas"></canvas>
    <div v-if="errorMsg" class="qr-error">{{ errorMsg }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import QRCode from 'qrcode'

const props = defineProps<{ url: string }>()
const canvas = ref<HTMLCanvasElement | null>(null)
const errorMsg = ref('')

async function render() {
  if (!canvas.value || !props.url) return
  errorMsg.value = ''
  try {
    await QRCode.toCanvas(canvas.value, props.url, {
      width: 200,
      margin: 2,
      color: { dark: '#e0e0e0', light: '#1a1a2e' },
    })
  } catch (e) {
    console.error('QR render failed:', e)
    errorMsg.value = 'Failed to generate QR code'
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
.qr-error {
  color: #ff6b6b;
  background: rgba(255, 107, 107, 0.1);
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-size: 0.85rem;
  text-align: center;
}
</style>
