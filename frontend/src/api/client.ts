const base = import.meta.env.VITE_API_URL || ''

const TOKEN_KEY = 'kanban_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export async function api<T>(
  path: string,
  init?: RequestInit & { json?: unknown }
): Promise<T> {
  const headers: Record<string, string> = {
    ...(init?.headers as Record<string, string>),
  }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  let body = init?.body
  if (init?.json !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(init.json)
  }
  const { json: _json, ...fetchInit } = (init ?? {}) as RequestInit & { json?: unknown }
  const r = await fetch(`${base}${path}`, { ...fetchInit, headers, body })
  if (r.status === 204) return undefined as T
  const text = await r.text()
  let data: { error?: string } | null = null
  if (text) {
    try {
      data = JSON.parse(text) as { error?: string }
    } catch {
      if (!r.ok) {
        throw new Error(text.slice(0, 200) || r.statusText || 'Request failed')
      }
    }
  }
  if (!r.ok) {
    const msg = data?.error || r.statusText || 'Request failed'
    throw new Error(msg)
  }
  return data as T
}

export function wsBoardUrl(boardId: string): string {
  const token = getToken()
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  if (base) {
    try {
      const u = new URL(base)
      const wsBase = `${u.protocol === 'https:' ? 'wss:' : 'ws:'}//${u.host}`
      return `${wsBase}/ws/boards/${boardId}?token=${encodeURIComponent(token || '')}`
    } catch {
      /* fall through */
    }
  }
  // Dev: Vite does not proxy WebSocket the same as fetch; hit backend directly.
  if (import.meta.env.DEV) {
    const host = window.location.hostname
    return `${proto}//${host}:8080/ws/boards/${boardId}?token=${encodeURIComponent(token || '')}`
  }
  // Production (e.g. Docker/nginx): same host as the SPA, path proxied to API.
  return `${proto}//${window.location.host}/ws/boards/${boardId}?token=${encodeURIComponent(token || '')}`
}
