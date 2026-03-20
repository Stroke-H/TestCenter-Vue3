// 工具函数集合

/**
 * 格式化日期时间
 * @param date - Date 对象或时间戳
 * @param fmt - 格式模板，默认 'YYYY-MM-DD HH:mm:ss'
 * @returns 格式化后的字符串
 */
export const formatDate = (date: Date | number, fmt = 'YYYY-MM-DD HH:mm:ss'): string => {
  const d = new Date(date)
  const map: Record<string, number> = {
    YYYY: d.getFullYear(),
    MM: d.getMonth() + 1,
    DD: d.getDate(),
    HH: d.getHours(),
    mm: d.getMinutes(),
    ss: d.getSeconds()
  }
  return fmt.replace(/YYYY|MM|DD|HH|mm|ss/g, (match) =>
    String(map[match]).padStart(2, '0')
  )
}

/**
 * 防抖函数
 * @param fn - 目标函数
 * @param delay - 延迟毫秒数
 */
export const debounce = <T extends (...args: unknown[]) => void>(fn: T, delay = 300) => {
  let timer: ReturnType<typeof setTimeout> | null = null
  return (...args: Parameters<T>) => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => fn(...args), delay)
  }
}
