import type { Env, PagesFunction, StoredSubscription } from '../types'
import { DEFAULT_SUBSCRIPTION_TTL, SUBSCRIPTION_TTLS } from '../config'

const MAX_CONTENT_BYTES = 256 * 1024

function json(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' },
  })
}

function randomToken(): string {
  const bytes = new Uint8Array(24)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export const onRequestPost: PagesFunction<Env> = async ({ request, env }) => {
  if (!env.SUBSCRIPTIONS) return json({ error: '订阅服务尚未配置 KV' }, 503)

  let body: { content?: unknown; ttl?: unknown }
  try {
    body = await request.json()
  } catch {
    return json({ error: '请求体必须是 JSON' }, 400)
  }

  if (typeof body.content !== 'string' || !body.content.trim()) {
    return json({ error: '缺少配置内容' }, 400)
  }
  if (new TextEncoder().encode(body.content).byteLength > MAX_CONTENT_BYTES) {
    return json({ error: '配置内容不能超过 256 KiB' }, 413)
  }

  if (body.ttl !== undefined &&
      (typeof body.ttl !== 'number' || !SUBSCRIPTION_TTLS.includes(body.ttl as typeof SUBSCRIPTION_TTLS[number]))) {
    return json({ error: '无效的订阅有效期' }, 400)
  }
  const ttl = body.ttl === undefined ? DEFAULT_SUBSCRIPTION_TTL : body.ttl
  const token = randomToken()
  const createdAt = new Date()
  const expiresAt = new Date(createdAt.getTime() + ttl * 1000)
  const record: StoredSubscription = {
    content: body.content,
    createdAt: createdAt.toISOString(),
    expiresAt: expiresAt.toISOString(),
  }

  await env.SUBSCRIPTIONS.put(token, JSON.stringify(record), { expirationTtl: ttl })
  const url = new URL(`/sub/${token}`, request.url).toString()
  return json({ url, token, expires_at: record.expiresAt, ttl })
}

export const onRequestGet: PagesFunction<Env> = async () =>
  json({ ttl_options: SUBSCRIPTION_TTLS, default_ttl: DEFAULT_SUBSCRIPTION_TTL })
