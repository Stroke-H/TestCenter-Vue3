export const ACCEPTANCE_REPORTS_CHANGED_EVENT = 'acceptance-reports-changed'
const ACCEPTANCE_REPORTS_CHANGED_KEY = 'acceptance_reports_changed_at'

export const notifyAcceptanceReportsChanged = () => {
  const changedAt = String(Date.now())
  window.dispatchEvent(new CustomEvent(ACCEPTANCE_REPORTS_CHANGED_EVENT, {
    detail: { changedAt }
  }))
  localStorage.setItem(ACCEPTANCE_REPORTS_CHANGED_KEY, changedAt)
}

export const isAcceptanceReportsChangedStorageKey = (key: string | null) => {
  return key === ACCEPTANCE_REPORTS_CHANGED_KEY
}
