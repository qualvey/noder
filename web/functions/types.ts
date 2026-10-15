export interface SubscriptionKV {
  get(key: string): Promise<string | null>
  put(key: string, value: string, options?: { expirationTtl?: number }): Promise<void>
}

export interface Env {
  SUBSCRIPTIONS?: SubscriptionKV
}

export interface StoredSubscription {
  content: string
  createdAt: string
  expiresAt: string
}

export type PagesFunction<E = Env> = (context: {
  request: Request
  env: E
  params: Record<string, string>
}) => Response | Promise<Response>
