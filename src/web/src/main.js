import { createApp } from 'vue'
import 'remixicon/fonts/remixicon.css'
import App from './App.vue'
import router from './router'
import { auth } from './auth'
import './styles.css'

auth.loadMe().finally(() => {
  createApp(App).use(router).mount('#app')
})
