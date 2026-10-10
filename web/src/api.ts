export interface Post {
  id: string
  username: string
  message: string
  mediaType: 'image' | 'video'
  mediaUrl: string
  createdAt: string
}

export interface Session {
  token: string
  username: string
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// The token lives in sessionStorage: it survives a page refresh but not
// closing the tab, and it is never put in a URL.
const KEY = 'socialai-session'

export function loadSession(): Session | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

export function saveSession(session: Session | null) {
  try {
    if (session) sessionStorage.setItem(KEY, JSON.stringify(session))
    else sessionStorage.removeItem(KEY)
  } catch {
    // Storage unavailable: the session just won't survive a refresh.
  }
}

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(path, { ...init, headers })
  const body = response.status === 204 ? undefined : await response.json().catch(() => undefined)
  if (!response.ok) {
    throw new ApiError(response.status, (body as { error?: string } | undefined)?.error ?? `Request failed (${response.status})`)
  }
  return body as T
}

const json = (body: unknown): RequestInit => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

export const api = {
  health: () => request<{ status: string; imageGeneration: boolean }>('/api/health'),
  signup: (username: string, password: string) => request<{ username: string }>('/api/signup', json({ username, password })),
  signin: (username: string, password: string) => request<Session>('/api/signin', json({ username, password })),
  posts: (token: string, filter: { q?: string; user?: string }) => {
    const params = new URLSearchParams()
    if (filter.q) params.set('q', filter.q)
    if (filter.user) params.set('user', filter.user)
    return request<Post[]>(`/api/posts?${params}`, {}, token)
  },
  createPost: (token: string, message: string, file: Blob, filename: string) => {
    const form = new FormData()
    form.append('message', message)
    form.append('media', file, filename)
    return request<Post>('/api/posts', { method: 'POST', body: form }, token)
  },
  deletePost: (token: string, id: string) => request<void>(`/api/posts/${id}`, { method: 'DELETE' }, token),
  generateImage: (token: string, prompt: string) => request<{ dataUrl: string }>('/api/ai/images', json({ prompt }), token),
}

/** Converts a data: URL returned by the image endpoint into an uploadable Blob. */
export async function dataUrlToBlob(dataUrl: string): Promise<Blob> {
  return (await fetch(dataUrl)).blob()
}
