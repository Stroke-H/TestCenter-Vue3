import type { NovelBook, NovelChapter } from '../types'

export type NovelEncoding = 'auto' | 'utf-8' | 'gb18030'

const CHAPTER_PATTERN =
  /^\s*((第\s*[0-9一二三四五六七八九十百千万零〇两]+\s*[章节卷回部篇集].*)|(chapter\s*\d+.*)|(序章|楔子|引子|前言|正文|尾声|后记|番外.*))\s*$/i

const MAX_TITLE_LENGTH = 60

export function createBookId(file: File): string {
  return `${file.name}_${file.size}_${file.lastModified}`
}

export function getDisplayName(fileName: string): string {
  return fileName.replace(/\.[^/.]+$/, '')
}

export async function readTextFile(file: File, encoding: NovelEncoding = 'auto'): Promise<string> {
  const buffer = await file.arrayBuffer()
  const text = encoding === 'auto'
    ? decodeNovelText(buffer)
    : decodeWithEncoding(buffer, encoding, false) ?? ''
  return normalizeText(text)
}

export function parseNovel(file: File, text: string): NovelBook {
  const chapters = parseChapters(text)

  return {
    id: createBookId(file),
    name: getDisplayName(file.name),
    fileName: file.name,
    fileSize: file.size,
    lastModified: file.lastModified,
    chapters,
    createdAt: Date.now()
  }
}

function normalizeText(text: string): string {
  return text
    .replace(/^\uFEFF/, '')
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .replace(/\u00a0/g, ' ')
}

function decodeNovelText(buffer: ArrayBuffer): string {
  const candidates = [
    decodeWithEncoding(buffer, 'utf-8', true),
    decodeWithEncoding(buffer, 'gb18030', false),
    decodeWithEncoding(buffer, 'gbk', false),
    decodeWithEncoding(buffer, 'utf-8', false)
  ].filter((item): item is string => Boolean(item))

  if (candidates.length === 0) {
    return ''
  }

  const bestCandidate = candidates
    .map((text) => ({ text, score: scoreDecodedText(text) }))
    .sort((a, b) => b.score - a.score)[0]

  return bestCandidate?.text ?? ''
}

function decodeWithEncoding(buffer: ArrayBuffer, encoding: string, fatal: boolean): string | null {
  try {
    return new TextDecoder(encoding, { fatal }).decode(buffer)
  } catch {
    return null
  }
}

function scoreDecodedText(text: string): number {
  const replacementCount = (text.match(/\uFFFD/g) ?? []).length
  const chineseCount = (text.match(/[\u4e00-\u9fa5]/g) ?? []).length
  const mojibakeCount = (text.match(/[锟斤拷ÃÂ]/g) ?? []).length
  const readableCount = (text.match(/[A-Za-z0-9，。！？、“”‘’：；（）《》\s]/g) ?? []).length

  return chineseCount * 4 + readableCount - replacementCount * 30 - mojibakeCount * 20
}

function parseChapters(text: string): NovelChapter[] {
  const lines = text.split('\n')
  const markers: Array<{ title: string; lineIndex: number }> = []

  lines.forEach((line, lineIndex) => {
    const title = line.trim()
    if (title.length > 0 && title.length <= MAX_TITLE_LENGTH && CHAPTER_PATTERN.test(title)) {
      markers.push({ title, lineIndex })
    }
  })

  if (markers.length === 0) {
    return [
      {
        id: 'chapter_0',
        title: '全文',
        content: cleanContent(text),
        startLine: 0
      }
    ]
  }

  const chapters: NovelChapter[] = []
  const firstMarker = markers[0]

  if (firstMarker && firstMarker.lineIndex > 0) {
    const prologue = cleanContent(lines.slice(0, firstMarker.lineIndex).join('\n'))
    if (prologue) {
      chapters.push({
        id: 'chapter_preface',
        title: '开篇',
        content: prologue,
        startLine: 0
      })
    }
  }

  markers.forEach((marker, index) => {
    const nextMarker = markers[index + 1]
    const start = marker.lineIndex + 1
    const end = nextMarker ? nextMarker.lineIndex : lines.length
    const content = cleanContent(lines.slice(start, end).join('\n'))

    chapters.push({
      id: `chapter_${index}`,
      title: marker.title,
      content,
      startLine: marker.lineIndex
    })
  })

  return chapters
}

function cleanContent(content: string): string {
  return content
    .split('\n')
    .map((line) => line.trimEnd())
    .join('\n')
    .replace(/\n{4,}/g, '\n\n\n')
    .trim()
}
