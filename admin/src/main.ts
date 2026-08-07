import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Await CSRF + session validation before the first navigation/render so the
  // router guard sees the correct login state.
  const auth = useAuthStore(pinia)
  await auth.init()

  app.use(router)

  app.config.errorHandler = (err, instance, info) => {
    console.error('Global error:', err, info)
  }

  app.mount('#app')
}

bootstrap()
