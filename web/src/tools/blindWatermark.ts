/**
 * 盲水印（不可见水印）工具：LSB 空域嵌入 / 提取
 *
 * 原理：
 * - 把载荷（魔数 + 长度 + 校验和 + 内容）的每个比特，按固定种子生成的
 *   伪随机像素位置，写入蓝色通道的最低位（可配 1~2 位），视觉上完全不可见。
 * - 同一比特重复写入 repeat 个位置；提取时按多数投票恢复，抗轻度干扰。
 * - 提取为真盲提取：不需要原图，任意图片可直接检测。
 *
 * 限制（已在面板提示）：
 * - PNG 无损导出可完整提取；JPEG 等有损压缩会破坏最低位，无法提取。
 * - 图片被缩放/裁剪/旋转后无法提取（像素位置序列失效）。
 * - 马赛克涂抹区域会覆盖该区域像素，仅损坏局部盲水印（多数投票可容忍）。
 */

export interface BlindWatermarkOptions {
  /** 每个像素写入的低位比特数：1 或 2，默认 1 */
  bits?: number
  /** 每比特重复写入次数（冗余），默认 5 */
  repeat?: number
}

const MAGIC = 'RWM1'
const MAGIC_BYTES = new TextEncoder().encode(MAGIC)
const DEFAULT_SEED = 0x9e3779b9

/** 确定性伪随机数（mulberry32），保证嵌入/提取位置序列一致 */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/**
 * 把文本嵌入 ImageData（就地修改像素）
 * @returns 是否嵌入成功（图片像素过少、内容过长时返回 false）
 */
export function embedBlindWatermark(
  data: ImageData,
  text: string,
  options: BlindWatermarkOptions = {},
): boolean {
  const bits = Math.min(2, Math.max(1, Math.round(options.bits ?? 1)))
  const repeat = Math.max(1, Math.round(options.repeat ?? 5))
  const payload = new TextEncoder().encode(text)
  if (payload.length > 0xffff) return false

  // 帧格式：MAGIC(4) + 长度(2, 大端) + 校验和(1, payload 逐字节异或) + payload
  const header = new Uint8Array(4 + 2 + 1 + payload.length)
  header.set(MAGIC_BYTES, 0)
  header[4] = payload.length >> 8
  header[5] = payload.length & 0xff
  let sum = 0
  for (const b of payload) sum ^= b
  header[6] = sum
  header.set(payload, 7)

  const { width, height, data: px } = data
  const n = width * height
  const totalBits = header.length * 8 * repeat
  if (totalBits > n) return false // 容量不足

  const rand = mulberry32(DEFAULT_SEED)
  const mask = (1 << bits) - 1
  const shift = bits - 1
  for (let b = 0; b < header.length; b++) {
    for (let bit = 0; bit < 8; bit++) {
      const v = ((header[b] ?? 0) >> (7 - bit)) & 1
      for (let r = 0; r < repeat; r++) {
        const off = Math.floor(rand() * n) * 4
        const cur = px[off + 2] ?? 0 // 蓝色通道（人眼最不敏感）
        px[off + 2] = (cur & ~mask) | ((v << shift) & mask)
      }
    }
  }
  return true
}

/**
 * 从 ImageData 提取盲水印文本（真盲提取，不需要原图）
 * @returns 提取到的文本；未检测到（魔数不匹配 / 校验失败）时返回 null
 */
export function extractBlindWatermark(
  data: ImageData,
  options: BlindWatermarkOptions = {},
): string | null {
  const bits = Math.min(2, Math.max(1, Math.round(options.bits ?? 1)))
  const repeat = Math.max(1, Math.round(options.repeat ?? 5))
  const { width, height, data: px } = data
  const n = width * height
  const rand = mulberry32(DEFAULT_SEED)
  const mask = (1 << bits) - 1
  const shift = bits - 1

  // 按嵌入顺序读取 count 个比特（每比特 repeat 次多数投票）
  const readBits = (count: number): number[] => {
    const out: number[] = []
    for (let i = 0; i < count; i++) {
      let votes = 0
      for (let r = 0; r < repeat; r++) {
        const off = Math.floor(rand() * n) * 4
        votes += ((px[off + 2] ?? 0) & mask) >> shift
      }
      out.push(votes > repeat / 2 ? 1 : 0)
    }
    return out
  }

  const bitsToBytes = (bitsArr: number[]): Uint8Array => {
    const bytes = new Uint8Array(bitsArr.length / 8)
    for (let i = 0; i < bytes.length; i++) {
      let b = 0
      for (let j = 0; j < 8; j++) b = (b << 1) | (bitsArr[i * 8 + j] ?? 0)
      bytes[i] = b
    }
    return bytes
  }

  // 帧头：MAGIC(4) + 长度(2) + 校验和(1)
  const headBits = readBits(7 * 8)
  const head = bitsToBytes(headBits)
  for (let i = 0; i < 4; i++) {
    if ((head[i] ?? 0) !== (MAGIC_BYTES[i] ?? 0)) return null
  }
  const len = ((head[4] ?? 0) << 8) | (head[5] ?? 0)
  if (len === 0) return null

  const payloadBits = readBits(len * 8)
  const payload = bitsToBytes(payloadBits)
  let sum = 0
  for (const b of payload) sum ^= b
  if (sum !== (head[6] ?? 0)) return null
  try {
    return new TextDecoder().decode(payload)
  } catch {
    return null
  }
}
