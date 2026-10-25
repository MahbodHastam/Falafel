import { createApp } from 'vue'
import '@fontsource-variable/vazirmatn'
import './style.css'

import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'

createApp(App).use(router).use(i18n).mount('#app')
