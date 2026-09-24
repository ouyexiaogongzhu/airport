import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  app.config.errorHandler = (err, instance, info) => {
    console.error('Global error:', err, info)
  }

  // Mount immediately with a dark boot shell so refresh isn't a white blank page.
  // Session restore still completes before route guards proceed (authReady).
  const auth = useAuthStore(pinia)
  void auth.init()

  app.use(router)
  app.mount('#app')
}

bootstrap()
