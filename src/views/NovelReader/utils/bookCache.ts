import type { NovelBook } from '../types'

const DB_NAME = 'novel_reader_cache'
const DB_VERSION = 1
const BOOK_STORE = 'books'

function openBookDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)

    request.onupgradeneeded = () => {
      const database = request.result
      if (!database.objectStoreNames.contains(BOOK_STORE)) {
        database.createObjectStore(BOOK_STORE, { keyPath: 'id' })
      }
    }

    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

async function runBookTransaction<T>(
  mode: IDBTransactionMode,
  handler: (store: IDBObjectStore) => IDBRequest<T>
): Promise<T> {
  const database = await openBookDatabase()

  return new Promise((resolve, reject) => {
    const transaction = database.transaction(BOOK_STORE, mode)
    const store = transaction.objectStore(BOOK_STORE)
    const request = handler(store)

    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
    transaction.oncomplete = () => database.close()
    transaction.onerror = () => {
      database.close()
      reject(transaction.error)
    }
  })
}

export function saveCachedBook(book: NovelBook) {
  return runBookTransaction('readwrite', (store) => store.put(book))
}

export function getCachedBook(bookId: string) {
  return runBookTransaction<NovelBook | undefined>('readonly', (store) => store.get(bookId))
}

export function deleteCachedBook(bookId: string) {
  return runBookTransaction('readwrite', (store) => store.delete(bookId))
}

export function clearCachedBooks() {
  return runBookTransaction('readwrite', (store) => store.clear())
}
