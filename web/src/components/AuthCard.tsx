import { useMutation } from '@tanstack/react-query'
import { App, Button, Card, Form, Input, Tabs } from 'antd'
import { api, type Session } from '../api'

interface Credentials {
  username: string
  password: string
}

export default function AuthCard({ onSignedIn }: { onSignedIn: (s: Session) => void }) {
  const { message } = App.useApp()

  const signin = useMutation({
    mutationFn: (c: Credentials) => api.signin(c.username, c.password),
    onSuccess: onSignedIn,
    onError: (e) => message.error(e.message),
  })
  const signup = useMutation({
    mutationFn: async (c: Credentials) => {
      await api.signup(c.username, c.password)
      return api.signin(c.username, c.password)
    },
    onSuccess: onSignedIn,
    onError: (e) => message.error(e.message),
  })

  const form = (onFinish: (c: Credentials) => void, loading: boolean, label: string, isSignup: boolean) => (
    <Form<Credentials> layout="vertical" onFinish={onFinish}>
      <Form.Item
        label="Username"
        name="username"
        rules={[
          { required: true },
          ...(isSignup ? [{ pattern: /^[a-z0-9_]{3,20}$/, message: '3-20 lowercase letters, digits or _' }] : []),
        ]}
      >
        <Input autoComplete="username" />
      </Form.Item>
      <Form.Item label="Password" name="password" rules={[{ required: true }, ...(isSignup ? [{ min: 8 }] : [])]}>
        <Input.Password autoComplete={isSignup ? 'new-password' : 'current-password'} />
      </Form.Item>
      <Button type="primary" htmlType="submit" block loading={loading}>
        {label}
      </Button>
    </Form>
  )

  return (
    <Card style={{ maxWidth: 400, margin: '48px auto' }}>
      <Tabs
        items={[
          { key: 'in', label: 'Sign in', children: form((c) => signin.mutate(c), signin.isPending, 'Sign in', false) },
          { key: 'up', label: 'Sign up', children: form((c) => signup.mutate(c), signup.isPending, 'Create account', true) },
        ]}
      />
    </Card>
  )
}
