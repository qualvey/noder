import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'

export type SearchFieldExtractor<T> = (item: T) => (string | number | boolean | null | undefined)[]

export interface UseFuzzySearchOptions<T> {
  fields: SearchFieldExtractor<T>
}

/**
 * 模糊搜索字符匹配：
 * 1. 直接包含（子串包含）
 * 2. 忽略常见分隔符（-、_、空格、点、冒号、斜杠）的容错匹配
 */
export function matchesFuzzy(text: string, term: string): boolean {
  if (text.includes(term)) return true
  const cleanText = text.replace(/[-_\s.:/]/g, '')
  const cleanTerm = term.replace(/[-_\s.:/]/g, '')
  return cleanTerm.length > 0 && cleanText.includes(cleanTerm)
}

/**
 * 通用多属性模糊搜索组合式函数 (Composable)
 *
 * @param items 数据源（支持 ref、computed、getter 或普通数组）
 * @param options 字段提取函数或配置选项
 */
export function useFuzzySearch<T>(
  items: MaybeRefOrGetter<T[]>,
  options: SearchFieldExtractor<T> | UseFuzzySearchOptions<T>
) {
  const searchQuery = ref('')
  const extractFields = typeof options === 'function' ? options : options.fields

  const filteredItems = computed(() => {
    const rawItems = toValue(items)
    const query = searchQuery.value.trim().toLowerCase()
    if (!query) return rawItems

    const terms = query.split(/\s+/).filter(Boolean)

    return rawItems.filter((item) => {
      const fieldValues = extractFields(item)
        .filter((v): v is string | number | boolean => v != null && v !== '')
        .map((v) => String(v).toLowerCase())

      const fullText = fieldValues.join(' ')

      return terms.every((term) =>
        fieldValues.some((f) => matchesFuzzy(f, term)) || matchesFuzzy(fullText, term)
      )
    })
  })

  function clear() {
    searchQuery.value = ''
  }

  return {
    searchQuery,
    filteredItems,
    clear,
  }
}
