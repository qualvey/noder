import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import globals from 'globals'

export default [
  { ignores: ['node_modules/**', 'dist/**', 'pages-dist/**', 'pages-public/**', '../static/**'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/base'],
  {
    files: ['src/**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: tseslint.parser,
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      // Vue 最佳实践：禁止直接变异 props（含 v-model 绑定 prop）
      'vue/no-mutating-props': 'error',
      // 禁止 v-html（XSS 风险，一律用插值或安全渲染）
      'vue/no-v-html': 'error',
      // 模板未使用变量
      'vue/no-unused-vars': 'error',
      // v-for 必须带 key
      'vue/require-v-for-key': 'error',
      // 避免 v-if 与 v-for 同节点
      'vue/no-use-v-if-with-v-for': 'error',
      // 重复 key / 重复属性
      'vue/no-dupe-keys': 'error',
      // TypeScript 最佳实践
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': 'off',
      'no-console': 'error',
      'no-debugger': 'error',
    },
  },
  {
    files: ['src/**/*.ts'],
    languageOptions: {
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': 'off',
      'no-console': 'error',
      'no-debugger': 'error',
    },
  },
]
