<template>
  <!-- TODO: Add skeleton loader components for improved perceived loading performance -->
  <router-view />
</template>

<script setup lang="ts">
import { onErrorCaptured } from 'vue'

// NOTE: Vue's template engine auto-escapes interpolated values ({{ }}),
// preventing XSS from user-supplied data. No additional sanitization is needed
// for template interpolation. Only use v-html with trusted content.

onErrorCaptured((err, _instance, info) => {
  console.error('[Admin Error Boundary]', info, err)
  return false // prevent propagation
})
</script>
