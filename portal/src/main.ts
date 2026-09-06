import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

async function bootstrap() {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  app.config.errorHandler = (err, instance, info) => {
    console.error('Global error:', err, info)
  }

  // 先恢復會話再掛 router：vue-router install() 會立即啟動首次導航，
  // 守衛必須在會話狀態就緒後才跑（否則刷新即被彈回登入頁）。
  const auth = useAuthStore(pinia)
  await auth.init()

  app.use(router)
  app.mount('#app')
}

bootstrap()
