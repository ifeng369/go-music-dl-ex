import { createApp } from 'vue'
import 'remixicon/fonts/remixicon.css'
import App from './App.vue'
import router from './router'
import { auth } from './auth'
import './styles.css'
import './netease.css'

// iOS/Safari 在封面加载失败时会显示破损图片图标，这里统一隐藏，交给容器背景兜底。
document.addEventListener(
  'error',
  (event) => {
    const target = event.target
    if (target instanceof HTMLImageElement) {
      target.classList.add('img-broken')
    }
  },
  true,
)

auth.loadMe().finally(() => {
  createApp(App).use(router).mount('#app')
})
