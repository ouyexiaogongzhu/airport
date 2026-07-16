<template>
  <div class="page coming-soon">
    <h1>Devices</h1>
    <p class="coming-soon-text">This page is coming soon.</p>
    <p class="coming-soon-hint">Manage your connected devices and sessions.</p>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.page {
  padding: 2rem;
  text-align: center;
}
.coming-soon {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}
.coming-soon h1 {
  color: #e2e8f0;
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
}
.coming-soon-text {
  color: #a0aec0;
  font-size: 1.1rem;
}
.coming-soon-hint {
  color: #718096;
  font-size: 0.9rem;
}
</style>
