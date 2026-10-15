// 管理员在这里配置前端可选择的订阅有效期（单位：秒）。
// Worker 会再次校验该数组，前端传入其它数值会被拒绝。
export const SUBSCRIPTION_TTLS = [3600] as const
export const DEFAULT_SUBSCRIPTION_TTL = 3600
