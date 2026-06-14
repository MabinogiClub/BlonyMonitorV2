/**
 * 工具函数
 * 提供格式化、转换等通用功能
 */

import { useAppStore } from '../stores/app'

/**
 * 颜色常量 - 用于图表和进度条
 */
export const COLORS = [
  '#ffc107', // 金色
  '#9c27b0', // 紫色
  '#009688', // 青色
  '#42a5f5', // 蓝色
  '#ff9800', // 橙色
  '#e91e63', // 粉色
  '#4caf50', // 绿色
  '#2196f3'  // 深蓝色
]

/**
 * 进度条样式类名
 */
export const BAR_CLASSES = [
  'bar-gold',
  'bar-purple',
  'bar-teal',
  'bar-blue',
  'bar-orange',
  'bar-pink'
]

/**
 * 格式化数字（K/M 缩写）
 * @param num 数字
 * @returns 格式化后的字符串
 */
export function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return Math.round(num).toString()
}

/**
 * HTML 转义，防止 XSS
 * @param text 原始文本
 * @returns 转义后的文本
 */
export function escapeHtml(text: string | undefined | null): string {
  if (!text) return ''
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

/**
 * 格式化时间戳
 * @param timestamp Unix 时间戳（秒）
 * @returns 格式化后的时间字符串
 */
export function formatTime(timestamp: number): string {
  const date = new Date(timestamp * 1000)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

/**
 * 获取技能名称
 * @param skillId 技能 ID
 * @returns 技能名称
 */
export function getSkillName(skillId: number): string {
  const appStore = useAppStore()
  return appStore.skillNameMap[skillId] || `技能${skillId}`
}

/**
 * 获取状态名称
 * @param conditionId 状态 ID
 * @returns 状态名称
 */
export function getConditionName(conditionId: number): string {
  const appStore = useAppStore()
  return appStore.conditionNameMap[conditionId] || `状态${conditionId}`
}

/**
 * 获取技能图标 URL (从数据库获取 base64)
 * @param skillId 技能 ID
 * @returns 图标 data URL 或空字符串
 */
export function getSkillIconUrl(skillId: number): string {
  const appStore = useAppStore()
  const base64Icon = appStore.skillIconMap[skillId]
  if (base64Icon) {
    return `data:image/png;base64,${base64Icon}`
  }
  return ''
}

/**
 * 根据状态名称获取颜色类
 * @param conditionName 状态名称
 * @returns CSS 类名
 */
export function getConditionColorClass(conditionName: string | undefined): string {
  if (!conditionName) return ''
  if (conditionName.includes('攻击')) return 'condition-attack'
  if (conditionName.includes('魔法')) return 'condition-magic'
  if (conditionName.includes('曲') || conditionName.includes('歌') || conditionName.includes('乐')) return 'condition-song'
  if (conditionName.includes('穿刺')) return 'condition-pierce'
  return ''
}

/**
 * 获取显示名称（从生物库或使用 ID）
 * @param id 实体 ID
 * @param name 实体名称
 * @returns 格式化后的显示名称
 */
export function getDisplayName(id: string, name: string | undefined): string {
  const shortId = id.length > 6 ? id.slice(-6) : id
  const isValidName = name && name !== id && name !== shortId
  if (isValidName) {
    return `${name}(${shortId})`
  }
  return `未知(${shortId})`
}

/**
 * 格式化时长（秒转为可读格式）
 * @param seconds 秒数
 * @returns 格式化后的时长字符串
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}秒`
  }
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (minutes < 60) {
    return remainingSeconds > 0 ? `${minutes}分${remainingSeconds}秒` : `${minutes}分`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}时${remainingMinutes}分`
}

/**
 * 防抖函数
 * @param fn 要防抖的函数
 * @param delay 延迟时间（毫秒）
 * @returns 防抖后的函数
 */
export function debounce<T extends (...args: any[]) => any>(fn: T, delay: number): T {
  let timeoutId: number | null = null
  return ((...args: Parameters<T>) => {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
    }
    timeoutId = window.setTimeout(() => {
      fn(...args)
      timeoutId = null
    }, delay)
  }) as T
}

/**
 * 节流函数
 * @param fn 要节流的函数
 * @param interval 最小间隔时间（毫秒）
 * @returns 节流后的函数
 */
export function throttle<T extends (...args: any[]) => any>(fn: T, interval: number): T {
  let lastTime = 0
  let pendingArgs: Parameters<T> | null = null
  let timeoutId: number | null = null

  return ((...args: Parameters<T>) => {
    const now = Date.now()
    const timeSinceLastCall = now - lastTime

    if (timeSinceLastCall >= interval) {
      // 已经超过间隔时间，立即执行
      lastTime = now
      fn(...args)
    } else {
      // 还没到间隔时间，保存参数，等待下次执行
      pendingArgs = args
      if (timeoutId === null) {
        timeoutId = window.setTimeout(() => {
          if (pendingArgs !== null) {
            lastTime = Date.now()
            fn(...pendingArgs)
            pendingArgs = null
          }
          timeoutId = null
        }, interval - timeSinceLastCall)
      }
    }
  }) as T
}
