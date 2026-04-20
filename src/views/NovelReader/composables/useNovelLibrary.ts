import { computed, reactive } from 'vue'
import type { ReaderSettings, ReadingRecord } from '../types'

const RECORDS_KEY = 'novel_reader_records'
const SETTINGS_KEY = 'novel_reader_settings'

const defaultSettings: ReaderSettings = {
  fontSize: 18,
  lineHeight: 'comfortable',
  width: 'standard',
  theme: 'soft',
  font: 'serif',
  indent: true,
  smoothScroll: true
}

function loadRecords(): ReadingRecord[] {
  const cached = localStorage.getItem(RECORDS_KEY)
  if (!cached) return []

  try {
    const parsed = JSON.parse(cached)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function loadSettings(): ReaderSettings {
  const cached = localStorage.getItem(SETTINGS_KEY)
  if (!cached) return { ...defaultSettings }

  try {
    return { ...defaultSettings, ...JSON.parse(cached) }
  } catch {
    return { ...defaultSettings }
  }
}

export function useNovelLibrary() {
  const records = reactive<ReadingRecord[]>(loadRecords())
  const settings = reactive<ReaderSettings>(loadSettings())

  const latestRecord = computed(() => records[0] ?? null)

  function saveRecords() {
    localStorage.setItem(RECORDS_KEY, JSON.stringify(records.slice(0, 50)))
  }

  function saveSettings() {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
  }

  function upsertRecord(record: ReadingRecord) {
    const existingIndex = records.findIndex((item) => item.id === record.id)
    if (existingIndex >= 0) {
      records.splice(existingIndex, 1)
    }

    records.unshift(record)

    if (records.length > 50) {
      records.splice(50)
    }

    saveRecords()
  }

  function findRecord(bookId: string) {
    return records.find((record) => record.id === bookId) ?? null
  }

  function clearRecords() {
    records.splice(0)
    localStorage.removeItem(RECORDS_KEY)
  }

  function removeRecord(bookId: string) {
    const index = records.findIndex((record) => record.id === bookId)
    if (index >= 0) {
      records.splice(index, 1)
      saveRecords()
    }
  }

  return {
    records,
    settings,
    latestRecord,
    saveSettings,
    upsertRecord,
    findRecord,
    removeRecord,
    clearRecords
  }
}
