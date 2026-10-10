import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { App as AntApp } from 'antd'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

function renderApp() {
  render(
    <AntApp>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <App />
      </QueryClientProvider>
    </AntApp>,
  )
}

beforeEach(() => sessionStorage.clear())
afterEach(() => vi.restoreAllMocks())

describe('App', () => {
  it('shows sign-in when there is no session', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ status: 'ok', imageGeneration: false })))
    renderApp()
    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('loads the feed with the bearer token and marks the user’s own posts', async () => {
    sessionStorage.setItem('drizzle-session', JSON.stringify({ token: 'tok', username: 'miles' }))
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = String(input)
      if (url === '/api/health') return new Response(JSON.stringify({ status: 'ok', imageGeneration: true }))
      if (url.startsWith('/api/posts')) {
        return new Response(
          JSON.stringify([
            { id: '1', username: 'miles', message: 'Hiking at Rainier', mediaType: 'image', mediaUrl: '/media/1.png', createdAt: '2026-10-09T00:00:00Z' },
            { id: '2', username: 'alice', message: 'Coffee', mediaType: 'image', mediaUrl: '/media/2.png', createdAt: '2026-10-08T00:00:00Z' },
          ]),
        )
      }
      return new Response(null, { status: 404 })
    })

    renderApp()

    expect(await screen.findByText('Hiking at Rainier')).toBeInTheDocument()
    expect(screen.getByText('@alice')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Delete post' })).toHaveLength(1)

    const postsCall = fetchSpy.mock.calls.find(([url]) => String(url).startsWith('/api/posts'))
    expect(new Headers(postsCall?.[1]?.headers).get('Authorization')).toBe('Bearer tok')
  })

  it('signs out when the token is rejected', async () => {
    sessionStorage.setItem('drizzle-session', JSON.stringify({ token: 'expired', username: 'miles' }))
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) =>
      String(input) === '/api/health'
        ? new Response(JSON.stringify({ status: 'ok', imageGeneration: false }))
        : new Response(JSON.stringify({ error: 'invalid or expired token' }), { status: 401 }),
    )
    renderApp()
    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(sessionStorage.getItem('drizzle-session')).toBeNull()
  })
})
