import type { NovelBook, NovelChapter } from '../types'

export type NovelEncoding =
  | 'auto'
  | 'utf-8'
  | 'gb18030'
  | 'gbk'
  | 'big5'
  | 'utf-16le'
  | 'utf-16be'
  | 'shift_jis'
  | 'euc-jp'
  | 'euc-kr'
  | 'windows-1252'
  | 'windows-1251'
  | 'iso-8859-1'

const CHAPTER_PATTERN =
  /^\s*((第\s*[0-9一二三四五六七八九十百千万零〇两]+\s*[章节卷回部篇集].*)|(chapter\s*\d+.*)|(序章|楔子|引子|前言|正文|尾声|后记|番外.*))\s*$/i

const MAX_TITLE_LENGTH = 60

export const NOVEL_ENCODING_OPTIONS: Array<{ label: string; value: NovelEncoding }> = [
  { label: '自动识别', value: 'auto' },
  { label: 'UTF-8', value: 'utf-8' },
  { label: 'GBK', value: 'gbk' },
  { label: 'GB18030', value: 'gb18030' },
  { label: 'Big5 繁体中文', value: 'big5' },
  { label: 'UTF-16 LE', value: 'utf-16le' },
  { label: 'UTF-16 BE', value: 'utf-16be' },
  { label: 'Shift_JIS 日文', value: 'shift_jis' },
  { label: 'EUC-JP 日文', value: 'euc-jp' },
  { label: 'EUC-KR 韩文', value: 'euc-kr' },
  { label: 'Windows-1252 西欧', value: 'windows-1252' },
  { label: 'Windows-1251 西里尔', value: 'windows-1251' },
  { label: 'ISO-8859-1 拉丁', value: 'iso-8859-1' }
]

export const SUPPORTED_NOVEL_FILE_TYPES = [
  '.txt',
  '.text',
  '.md',
  '.markdown',
  '.log',
  '.html',
  '.htm',
  '.xhtml',
  '.xml',
  '.epub'
]

export const SUPPORTED_NOVEL_FILE_ACCEPT = [
  ...SUPPORTED_NOVEL_FILE_TYPES,
  'text/plain',
  'text/markdown',
  'text/html',
  'application/xhtml+xml',
  'application/xml',
  'application/epub+zip'
].join(',')

export function createBookId(file: File): string {
  return `${file.name}_${file.size}_${file.lastModified}`
}

export function getDisplayName(fileName: string): string {
  return fileName.replace(/\.[^/.]+$/, '')
}

export async function readTextFile(file: File, encoding: NovelEncoding = 'auto'): Promise<string> {
  const buffer = await file.arrayBuffer()
  const extension = getFileExtension(file.name)

  if (extension === '.epub') {
    return normalizeText(await readEpubText(buffer))
  }

  const text = encoding === 'auto'
    ? decodeNovelText(buffer)
    : decodeWithEncoding(buffer, encoding, false) ?? ''
  const normalizedText = normalizeText(text)

  return isMarkupExtension(extension) ? stripMarkup(normalizedText) : normalizedText
}

export function isSupportedNovelFile(file: File) {
  const extension = getFileExtension(file.name)
  return SUPPORTED_NOVEL_FILE_TYPES.includes(extension) || isSupportedTextMime(file.type)
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
    decodeWithEncoding(buffer, 'big5', false),
    decodeWithEncoding(buffer, 'utf-16le', false),
    decodeWithEncoding(buffer, 'utf-16be', false),
    decodeWithEncoding(buffer, 'shift_jis', false),
    decodeWithEncoding(buffer, 'euc-jp', false),
    decodeWithEncoding(buffer, 'euc-kr', false),
    decodeWithEncoding(buffer, 'windows-1252', false),
    decodeWithEncoding(buffer, 'windows-1251', false),
    decodeWithEncoding(buffer, 'iso-8859-1', false),
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
  const japaneseCount = (text.match(/[\u3040-\u30ff]/g) ?? []).length
  const koreanCount = (text.match(/[\uac00-\ud7af]/g) ?? []).length
  const latinCount = (text.match(/[A-Za-zÀ-ÿ]/g) ?? []).length
  const mojibakeCount = (text.match(/[锟斤拷ÃÂ]/g) ?? []).length
  const readableCount = (text.match(/[A-Za-z0-9，。！？、“”‘’：；（）《》\s]/g) ?? []).length

  return chineseCount * 4 + japaneseCount * 3 + koreanCount * 3 + latinCount + readableCount - replacementCount * 30 - mojibakeCount * 20
}

function getFileExtension(fileName: string) {
  const match = fileName.toLowerCase().match(/\.[^.]+$/)
  return match?.[0] ?? ''
}

function isSupportedTextMime(mimeType: string) {
  return mimeType.startsWith('text/') || [
    'application/xml',
    'application/xhtml+xml',
    'application/epub+zip'
  ].includes(mimeType)
}

function isMarkupExtension(extension: string) {
  return ['.html', '.htm', '.xhtml', '.xml'].includes(extension)
}

function stripMarkup(text: string) {
  return text
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<\/(p|div|section|article|h[1-6]|br|li)>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

async function readEpubText(buffer: ArrayBuffer) {
  const entries = await readZipEntries(buffer)
  const textEntries = await getOrderedEpubTextEntries(entries)
  const parts = await Promise.all(textEntries.map(async (entryName) => {
    const entry = entries.get(entryName)
    if (!entry) return ''
    const data = await inflateZipEntry(entry)
    return stripMarkup(new TextDecoder('utf-8').decode(data))
  }))

  return parts.filter(Boolean).join('\n\n')
}

interface ZipEntry {
  name: string
  method: number
  compressedSize: number
  uncompressedSize: number
  localHeaderOffset: number
  buffer: ArrayBuffer
}

async function readZipEntries(buffer: ArrayBuffer) {
  const view = new DataView(buffer)
  const entries = new Map<string, ZipEntry>()
  const endOffset = findEndOfCentralDirectory(view)
  if (endOffset < 0) throw new Error('Invalid EPUB file')

  const centralDirectorySize = view.getUint32(endOffset + 12, true)
  const centralDirectoryOffset = view.getUint32(endOffset + 16, true)
  const centralDirectoryEnd = centralDirectoryOffset + centralDirectorySize
  let offset = centralDirectoryOffset

  while (offset < centralDirectoryEnd && view.getUint32(offset, true) === 0x02014b50) {
    const method = view.getUint16(offset + 10, true)
    const compressedSize = view.getUint32(offset + 20, true)
    const uncompressedSize = view.getUint32(offset + 24, true)
    const nameLength = view.getUint16(offset + 28, true)
    const extraLength = view.getUint16(offset + 30, true)
    const commentLength = view.getUint16(offset + 32, true)
    const localHeaderOffset = view.getUint32(offset + 42, true)
    const nameBytes = new Uint8Array(buffer, offset + 46, nameLength)
    const name = new TextDecoder('utf-8').decode(nameBytes)

    entries.set(name, { name, method, compressedSize, uncompressedSize, localHeaderOffset, buffer })
    offset += 46 + nameLength + extraLength + commentLength
  }

  return entries
}

function findEndOfCentralDirectory(view: DataView) {
  const minOffset = Math.max(0, view.byteLength - 65557)
  for (let offset = view.byteLength - 22; offset >= minOffset; offset -= 1) {
    if (view.getUint32(offset, true) === 0x06054b50) return offset
  }
  return -1
}

async function getOrderedEpubTextEntries(entries: Map<string, ZipEntry>) {
  const allTextEntries = getAllEpubTextEntries(entries)
  const container = entries.get('META-INF/container.xml')

  if (!container) return allTextEntries

  const containerText = new TextDecoder('utf-8').decode(await inflateZipEntry(container))
  const opfPath = containerText.match(/full-path=["']([^"']+)["']/i)?.[1]
  const opfEntry = opfPath ? entries.get(opfPath) : undefined
  if (!opfEntry || !opfPath) return allTextEntries

  const opfText = new TextDecoder('utf-8').decode(await inflateZipEntry(opfEntry))
  const opfBasePath = opfPath.includes('/') ? `${opfPath.slice(0, opfPath.lastIndexOf('/'))}/` : ''
  const manifest = new Map<string, string>()
  const manifestPattern = /<item\b[^>]*\bid=["']([^"']+)["'][^>]*\bhref=["']([^"']+)["'][^>]*>/gi
  let manifestMatch = manifestPattern.exec(opfText)

  while (manifestMatch) {
    const [, id, href] = manifestMatch
    if (id && href) {
      manifest.set(id, normalizeZipPath(`${opfBasePath}${decodeURIComponent(href)}`))
    }
    manifestMatch = manifestPattern.exec(opfText)
  }

  const orderedEntries: string[] = []
  const spinePattern = /<itemref\b[^>]*\bidref=["']([^"']+)["'][^>]*>/gi
  let spineMatch = spinePattern.exec(opfText)

  while (spineMatch) {
    const idref = spineMatch[1]
    const entryName = idref ? manifest.get(idref) : undefined
    if (entryName && entries.has(entryName) && /\.(xhtml|html|htm)$/i.test(entryName)) {
      orderedEntries.push(entryName)
    }
    spineMatch = spinePattern.exec(opfText)
  }

  return orderedEntries.length > 0 ? orderedEntries : allTextEntries
}

function getAllEpubTextEntries(entries: Map<string, ZipEntry>) {
  return Array.from(entries.keys())
    .filter((name) => /\.(xhtml|html|htm)$/i.test(name))
    .sort((a, b) => a.localeCompare(b))
}

function normalizeZipPath(path: string) {
  const segments: string[] = []
  path.split('/').forEach((segment) => {
    if (!segment || segment === '.') return
    if (segment === '..') {
      segments.pop()
      return
    }
    segments.push(segment)
  })
  return segments.join('/')
}

async function inflateZipEntry(entry: ZipEntry) {
  const view = new DataView(entry.buffer)
  const nameLength = view.getUint16(entry.localHeaderOffset + 26, true)
  const extraLength = view.getUint16(entry.localHeaderOffset + 28, true)
  const dataOffset = entry.localHeaderOffset + 30 + nameLength + extraLength
  const compressed = entry.buffer.slice(dataOffset, dataOffset + entry.compressedSize)

  if (entry.method === 0) return new Uint8Array(compressed)
  if (entry.method !== 8 || typeof DecompressionStream === 'undefined') {
    throw new Error(`Unsupported EPUB compression method: ${entry.method}`)
  }

  const stream = new Blob([compressed]).stream().pipeThrough(new DecompressionStream('deflate-raw' as CompressionFormat))
  return new Uint8Array(await new Response(stream).arrayBuffer())
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
