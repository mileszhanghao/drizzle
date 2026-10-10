import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App as AntApp, Button, Col, Empty, Input, Layout, Row, Segmented, Space, Spin, Typography } from 'antd'
import { useCallback, useState } from 'react'
import { ApiError, api, loadSession, saveSession, type Session } from './api'
import AuthCard from './components/AuthCard'
import CreatePostModal from './components/CreatePostModal'
import PostCard from './components/PostCard'

const { Header, Content } = Layout

export default function App() {
  const queryClient = useQueryClient()
  const { message } = AntApp.useApp()
  const [session, setSession] = useState<Session | null>(loadSession)
  const [scope, setScope] = useState<'all' | 'mine'>('all')
  const [search, setSearch] = useState('')
  const [creating, setCreating] = useState(false)

  const signOut = useCallback(() => {
    saveSession(null)
    setSession(null)
    queryClient.removeQueries({ queryKey: ['posts'] })
  }, [queryClient])

  const health = useQuery({ queryKey: ['health'], queryFn: api.health })
  const posts = useQuery({
    queryKey: ['posts', session?.token, scope, search],
    queryFn: async () => {
      try {
        return await api.posts(session!.token, { q: search, user: scope === 'mine' ? session!.username : undefined })
      } catch (error) {
        // An expired or invalid token sends the user back to sign-in.
        if (error instanceof ApiError && error.status === 401) {
          message.warning('Your session expired, please sign in again')
          signOut()
        }
        throw error
      }
    },
    enabled: Boolean(session),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.deletePost(session!.token, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['posts'] }),
    onError: (e) => message.error(e.message),
  })

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
        <Typography.Title level={3} style={{ color: '#fff', margin: 0 }}>
          SocialAI
        </Typography.Title>
        {session && (
          <Space>
            <Typography.Text style={{ color: '#fff' }}>@{session.username}</Typography.Text>
            <Button onClick={signOut}>Sign out</Button>
          </Space>
        )}
      </Header>
      <Content style={{ padding: '24px 16px', maxWidth: 1100, width: '100%', margin: '0 auto', boxSizing: 'border-box' }}>
        {!session ? (
          <AuthCard
            onSignedIn={(s) => {
              saveSession(s)
              setSession(s)
            }}
          />
        ) : (
          <Space orientation="vertical" size="large" style={{ width: '100%' }}>
            <Space wrap style={{ width: '100%', justifyContent: 'space-between' }}>
              <Space wrap>
                <Segmented
                  value={scope}
                  onChange={(v) => setScope(v as 'all' | 'mine')}
                  options={[
                    { label: 'All posts', value: 'all' },
                    { label: 'My posts', value: 'mine' },
                  ]}
                />
                <Input.Search
                  allowClear
                  placeholder="Search captions (e.g. hiking -rain)"
                  onSearch={setSearch}
                  style={{ width: 280 }}
                />
              </Space>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreating(true)}>
                New post
              </Button>
            </Space>

            {posts.isPending ? (
              <Spin />
            ) : posts.data?.length ? (
              <Row gutter={[16, 16]}>
                {posts.data.map((post) => (
                  <Col key={post.id} xs={24} sm={12} md={8} lg={6}>
                    <PostCard post={post} mine={post.username === session.username} onDelete={(id) => remove.mutate(id)} />
                  </Col>
                ))}
              </Row>
            ) : (
              <Empty description={search ? 'No posts match your search' : 'No posts yet'} />
            )}

            <CreatePostModal
              open={creating}
              token={session.token}
              aiEnabled={Boolean(health.data?.imageGeneration)}
              onClose={() => setCreating(false)}
            />
          </Space>
        )}
      </Content>
    </Layout>
  )
}
