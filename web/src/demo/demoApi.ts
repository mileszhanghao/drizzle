// In-browser stand-in for the Go API, used by the GitHub Pages demo. It mirrors
// the server's rules (validation, error messages, who can delete what) and keeps
// everything in this browser's localStorage. Passwords are stored as salted
// SHA-256 hashes even here, since some people reuse real passwords in demos.
import { ApiError, type Api, type Post, type Session } from '../api'
import { placeholderArt } from './placeholderArt'
import { DEMO_PASSWORD, DEMO_USER, seedPosts } from './seed'

const KEY = 'drizzle-demo-v1'
const USERNAME = /^[a-z0-9_]{3,20}$/
const MAX_IMAGE_SIDE = 1080

interface Account {
  salt: string
  hash: string
}

interface State {
  accounts: Record<string, Account>
  posts: Post[]
}

/** Turns an uploaded image into a resized JPEG data URL small enough for localStorage. */
export type ImageResizer = (file: Blob) => Promise<string>

async function sha256(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('')
}

async function makeAccount(password: string): Promise<Account> {
  const salt = crypto.randomUUID()
  return { salt, hash: await sha256(`${salt}:${password}`) }
}

async function freshState(): Promise<State> {
  return { accounts: { [DEMO_USER]: await makeAccount(DEMO_PASSWORD) }, posts: seedPosts() }
}

async function load(): Promise<State> {
  try {
    const raw = localStorage.getItem(KEY)
    if (raw) return JSON.parse(raw) as State
  } catch {
    // Storage blocked or corrupted: fall through to a fresh, in-memory state.
  }
  const state = await freshState()
  save(state)
  return state
}

function save(state: State) {
  try {
    localStorage.setItem(KEY, JSON.stringify(state))
  } catch {
    throw new ApiError(507, 'Demo storage in this browser is full. Use "Reset demo" to start over.')
  }
}

export function resetDemo() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // Nothing stored.
  }
}

function userFor(state: State, token: string): string {
  const username = token.startsWith('demo:') ? token.slice(5) : ''
  if (!state.accounts[username]) throw new ApiError(401, 'invalid or expired token')
  return username
}

/** Same idea as PostgreSQL's websearch_to_tsquery: words must all match, -word excludes. */
export function matches(message: string, q: string): boolean {
  const text = message.toLowerCase()
  const terms = q.toLowerCase().match(/-?"[^"]+"|-?\S+/g) ?? []
  return terms.every((raw) => {
    const negative = raw.startsWith('-')
    const term = raw.replace(/^-/, '').replace(/"/g, '').trim()
    if (!term) return true
    return text.includes(term) !== negative
  })
}

/** Browser implementation: decode, scale down to MAX_IMAGE_SIDE, re-encode as JPEG. */
export const canvasResizer: ImageResizer = async (file) => {
  const url = URL.createObjectURL(file)
  try {
    const img = new Image()
    img.src = url
    await img.decode()
    const scale = Math.min(1, MAX_IMAGE_SIDE / Math.max(img.naturalWidth, img.naturalHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(img.naturalWidth * scale)
    canvas.height = Math.round(img.naturalHeight * scale)
    canvas.getContext('2d')!.drawImage(img, 0, 0, canvas.width, canvas.height)
    return canvas.toDataURL('image/jpeg', 0.85)
  } catch {
    throw new ApiError(415, 'Could not read this image')
  } finally {
    URL.revokeObjectURL(url)
  }
}

export function createDemoApi(resize: ImageResizer = canvasResizer): Api {
  return {
    health: async () => ({ status: 'ok', imageGeneration: true, demo: true }),

    signup: async (username, password) => {
      if (!USERNAME.test(username)) throw new ApiError(400, 'username must be 3-20 lowercase letters, digits or underscores')
      if (password.length < 8 || password.length > 72) throw new ApiError(400, 'password must be 8-72 characters')
      const state = await load()
      if (state.accounts[username] || state.posts.some((p) => p.username === username)) {
        throw new ApiError(409, 'username already taken')
      }
      state.accounts[username] = await makeAccount(password)
      save(state)
      return { username }
    },

    signin: async (username, password): Promise<Session> => {
      const state = await load()
      const account = state.accounts[username]
      if (!account || (await sha256(`${account.salt}:${password}`)) !== account.hash) {
        throw new ApiError(401, 'wrong username or password')
      }
      return { token: `demo:${username}`, username }
    },

    posts: async (token, { q, user }) => {
      const state = await load()
      userFor(state, token)
      return state.posts
        .filter((p) => (!user || p.username === user) && (!q?.trim() || matches(p.message, q)))
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
    },

    createPost: async (token, message, file) => {
      const state = await load()
      const username = userFor(state, token)
      if (message.length > 2000) throw new ApiError(400, 'message is longer than 2000 characters')
      if (!file.type.startsWith('image/')) {
        throw new ApiError(415, 'The demo keeps posts in your browser, so it accepts images only. The full app also takes MP4 and WebM video.')
      }
      const post: Post = {
        id: crypto.randomUUID(),
        username,
        message: message.trim(),
        mediaType: 'image',
        mediaUrl: await resize(file),
        createdAt: new Date().toISOString(),
      }
      state.posts.push(post)
      save(state)
      return post
    },

    deletePost: async (token, id) => {
      const state = await load()
      const username = userFor(state, token)
      const index = state.posts.findIndex((p) => p.id === id && p.username === username)
      // Like the server: someone else's post looks the same as a missing one.
      if (index === -1) throw new ApiError(404, 'post not found')
      state.posts.splice(index, 1)
      save(state)
    },

    generateImage: async (token, prompt) => {
      userFor(await load(), token)
      const text = prompt.trim()
      if (!text || text.length > 1000) throw new ApiError(400, 'prompt must be 1-1000 characters')
      return { dataUrl: placeholderArt(text) }
    },
  }
}

export const demoApi = createDemoApi()
