import { createI18n } from 'vue-i18n'
import zh from './locales/zh-CN'
import en from './locales/en-US'

export type LocaleType = 'zh' | 'en'

const savedLocale = localStorage.getItem('noder_locale') as LocaleType | null
const defaultLocale: LocaleType = savedLocale || (navigator.language.toLowerCase().startsWith('en') ? 'en' : 'zh')

// 同步设置 HTML lang 属性
document.documentElement.lang = defaultLocale === 'zh' ? 'zh-CN' : 'en-US'

export const i18n = createI18n({
  legacy: false,
  locale: defaultLocale,
  fallbackLocale: 'zh',
  messages: {
    zh,
    en,
  },
})

export function setLocale(lang: LocaleType) {
  i18n.global.locale.value = lang
  localStorage.setItem('noder_locale', lang)
  document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en-US'
}
