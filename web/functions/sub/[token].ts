import type { Env, PagesFunction, StoredSubscription } from '../types'

export const onRequestGet: PagesFunction<Env> = async ({ env, params }) => {
  if (!env.SUBSCRIPTIONS) {
    return new Response('订阅服务尚未配置 KV', { status: 503 })
  }

  const token = params.token
  if (!token || !/^[a-f0-9]{48}$/.test(token)) {
    return new Response('订阅链接无效或已过期', { status: 404 })
  }
  const value = await env.SUBSCRIPTIONS.get(token)
  if (!value) return new Response('订阅链接无效或已过期', { status: 404 })

  let record: StoredSubscription
  try {
    record = JSON.parse(value) as StoredSubscription
  } catch {
    return new Response('订阅内容损坏', { status: 500 })
  }

  return new Response(record.content, {
    headers: {
      'content-type': 'application/json; charset=utf-8',
      'cache-control': 'no-store, no-cache, must-revalidate',
      'content-disposition': 'inline; filename="sing-box-subscription.json"',
    },
  })
}
