import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)
  app.use(router)

  app.config.errorHandler = (err, instance, info) => {
    console.error('Global error:', err, info)
  }

  // Restore the session from the httpOnly cookie before the first render, so
  // the router guard and UI start from the correct auth state.
  const auth = useAuthStore(pinia)
  await auth.init()

  app.mount('#app')
}

bootstrap()
