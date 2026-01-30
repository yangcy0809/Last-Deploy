export class ApiError extends Error {
  readonly status: number
  readonly body: unknown

  constructor(message: string, status: number, body: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '/api').replace(/\/$/, '')

function urlFor(path: string): string {
  if (!path.startsWith('/')) {
    throw new Error(`API path must start with "/": ${path}`)
  }
  return `${API_BASE}${path}`
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('accept', 'application/json')
  if (init.body != null && !headers.has('content-type')) {
    headers.set('content-type', 'application/json')
  }

  const res = await fetch(urlFor(path), { ...init, headers })

  const contentType = res.headers.get('content-type') ?? ''
  const isJson = contentType.includes('application/json')

  const body: unknown = isJson
    ? await res.json().catch(() => null)
    : await res.text().catch(() => null)

  if (!res.ok) {
    const message =
      typeof body === 'object' && body && 'error' in body
        ? String((body as { error: unknown }).error)
        : `HTTP ${res.status}`
    throw new ApiError(message, res.status, body)
  }

  return body as T
}

