import { beforeEach, describe, expect, it } from 'vitest'
import { ApiError } from '../api'
import { createDemoApi, matches, resetDemo } from './demoApi'
import { placeholderArt } from './placeholderArt'
import { DEMO_PASSWORD, DEMO_USER } from './seed'

const api = createDemoApi(async () => 'data:image/jpeg;base64,AAAA')
const png = new Blob(['x'], { type: 'image/png' })

beforeEach(() => resetDemo())

describe('demo API', () => {
  it('signs in the demo user and lists seeded posts newest first', async () => {
    const { token } = await api.signin(DEMO_USER, DEMO_PASSWORD)
    const posts = await api.posts(token, {})
    expect(posts).toHaveLength(8)
    expect(posts[0]!.createdAt > posts[1]!.createdAt).toBe(true)
    expect(posts.every((p) => p.credit?.license)).toBe(true)
  })

  it('rejects wrong passwords, bad usernames and taken names like the server', async () => {
    await expect(api.signin(DEMO_USER, 'nope-nope')).rejects.toMatchObject({ status: 401 })
    await expect(api.signup('Bad Name', 'longenough')).rejects.toMatchObject({ status: 400 })
    await expect(api.signup('ab', 'longenough')).rejects.toMatchObject({ status: 400 })
    await expect(api.signup('newbie', 'short')).rejects.toMatchObject({ status: 400 })
    await expect(api.signup('ferry_commuter', 'longenough')).rejects.toMatchObject({ status: 409 })
    await expect(api.posts('demo:ghost', {})).rejects.toBeInstanceOf(ApiError)
  })

  it('searches captions with websearch-style terms', async () => {
    const { token } = await api.signin(DEMO_USER, DEMO_PASSWORD)
    const rain = await api.posts(token, { q: 'rain' })
    const rainNoGasWorks = await api.posts(token, { q: 'rain -kites' })
    expect(rain.length).toBeGreaterThan(rainNoGasWorks.length)
    expect(rainNoGasWorks.some((p) => p.username === 'gasworks_kites')).toBe(false)
    expect(matches('Ferry across Elliott Bay', '"elliott bay" -fog')).toBe(true)
  })

  it('creates, filters and deletes your own posts, but not other people\'s', async () => {
    await api.signup('newbie', 'longenough')
    const { token } = await api.signin('newbie', 'longenough')
    const post = await api.createPost(token, 'First drizzle of the season', png, 'a.png')
    expect(await api.posts(token, { user: 'newbie' })).toEqual([post])

    await expect(api.createPost(token, 'clip', new Blob(['x'], { type: 'video/mp4' }), 'a.mp4')).rejects.toMatchObject({ status: 415 })
    await expect(api.deletePost(token, 'seed-1')).rejects.toMatchObject({ status: 404 })
    await api.deletePost(token, post.id)
    expect(await api.posts(token, { user: 'newbie' })).toEqual([])
  })

  it('draws a deterministic placeholder for image prompts', async () => {
    const { token } = await api.signin(DEMO_USER, DEMO_PASSWORD)
    const { dataUrl } = await api.generateImage(token, 'a corgi in a raincoat <at> Pike Place')
    expect(dataUrl).toBe(placeholderArt('a corgi in a raincoat <at> Pike Place'))
    expect(decodeURIComponent(dataUrl)).toContain('&#60;at&#62;')
    await expect(api.generateImage(token, '   ')).rejects.toMatchObject({ status: 400 })
  })
})
