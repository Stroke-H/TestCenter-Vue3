export interface NovelChapter {
  id: string
  title: string
  content: string
  startLine: number
}

export interface NovelBook {
  id: string
  name: string
  fileName: string
  fileSize: number
  lastModified: number
  chapters: NovelChapter[]
  createdAt: number
}

export type ReaderTheme = 'day' | 'soft' | 'green' | 'night' | 'black'
export type ReaderFont = 'system' | 'serif' | 'sans'
export type ReaderWidth = 'narrow' | 'standard' | 'wide'
export type ReaderLineHeight = 'compact' | 'standard' | 'comfortable' | 'loose'

export interface ReaderSettings {
  fontSize: number
  lineHeight: ReaderLineHeight
  width: ReaderWidth
  theme: ReaderTheme
  font: ReaderFont
  indent: boolean
  smoothScroll: boolean
}

export interface ReadingRecord {
  id: string
  name: string
  fileName: string
  fileSize: number
  lastModified: number
  chapterIndex: number
  scrollTop: number
  chapterCount: number
  progress: number
  updatedAt: number
}
