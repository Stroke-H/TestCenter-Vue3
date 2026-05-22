const BACKEND_PORT = import.meta.env.VITE_BACKEND_PORT || '8080'
const BACKEND_ORIGIN = import.meta.env.VITE_BACKEND_ORIGIN || ''

const trimTrailingSlash = (value: string) => value.replace(/\/+$/, '')

const isLocalBackendHost = (hostname: string) => {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '0.0.0.0'
}

const isBackendOwnedPath = (pathname: string) => {
  return (
    pathname.startsWith('/reports/') ||
    pathname.startsWith('/performance-reports/') ||
    pathname.startsWith('/api/test-runs/')
  )
}

export const getBackendOrigin = () => {
  if (BACKEND_ORIGIN) {
    return trimTrailingSlash(BACKEND_ORIGIN)
  }

  return `${window.location.protocol}//${window.location.hostname}:${BACKEND_PORT}`
}

export const getBackendWsOrigin = () => {
  const backendOrigin = new URL(getBackendOrigin())
  backendOrigin.protocol = backendOrigin.protocol === 'https:' ? 'wss:' : 'ws:'
  return trimTrailingSlash(backendOrigin.toString())
}

export const buildBackendUrl = (path: string) => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${getBackendOrigin()}${normalizedPath}`
}

export const buildBackendWsUrl = (path: string) => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${getBackendWsOrigin()}${normalizedPath}`
}

export const normalizeBackendUrl = (value: string) => {
  if (!value) return value

  if (isBackendOwnedPath(value)) {
    return buildBackendUrl(value)
  }

  try {
    const parsedUrl = new URL(value, window.location.origin)

    if (isBackendOwnedPath(parsedUrl.pathname)) {
      const backendOrigin = new URL(getBackendOrigin())
      parsedUrl.protocol = backendOrigin.protocol
      parsedUrl.hostname = backendOrigin.hostname
      parsedUrl.port = backendOrigin.port
      return parsedUrl.toString()
    }

    if (parsedUrl.origin === window.location.origin) {
      return parsedUrl.toString()
    }

    if (isLocalBackendHost(parsedUrl.hostname) && (!parsedUrl.port || parsedUrl.port === BACKEND_PORT)) {
      const backendOrigin = new URL(getBackendOrigin())
      parsedUrl.protocol = backendOrigin.protocol
      parsedUrl.hostname = backendOrigin.hostname
      parsedUrl.port = backendOrigin.port
    }

    return parsedUrl.toString()
  } catch {
    return value
  }
}
