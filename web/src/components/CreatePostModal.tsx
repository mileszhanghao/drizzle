import { InboxOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { App, Button, Image, Input, Modal, Space, Tabs, Typography, Upload } from 'antd'
import { useState } from 'react'
import { api, dataUrlToBlob } from '../api'

interface Props {
  open: boolean
  token: string
  aiEnabled: boolean
  /** GitHub Pages demo: images only, and Generate draws a placeholder. */
  demo?: boolean
  onClose: () => void
}

export default function CreatePostModal({ open, token, aiEnabled, demo = false, onClose }: Props) {
  const queryClient = useQueryClient()
  const { message } = App.useApp()
  const [caption, setCaption] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [prompt, setPrompt] = useState('')
  const [generated, setGenerated] = useState<string | null>(null)
  const [tab, setTab] = useState('upload')

  const reset = () => {
    setCaption('')
    setFile(null)
    setPrompt('')
    setGenerated(null)
  }

  const generate = useMutation({
    mutationFn: () => api.generateImage(token, prompt),
    onSuccess: ({ dataUrl }) => setGenerated(dataUrl),
    onError: (e) => message.error(e.message),
  })

  const publish = useMutation({
    mutationFn: async () => {
      if (tab === 'ai' && generated) return api.createPost(token, caption, await dataUrlToBlob(generated), 'generated.png')
      if (file) return api.createPost(token, caption, file, file.name)
      throw new Error('Choose a file or generate an image first')
    },
    onSuccess: () => {
      message.success('Posted')
      queryClient.invalidateQueries({ queryKey: ['posts'] })
      reset()
      onClose()
    },
    onError: (e) => message.error(e.message),
  })

  const ready = tab === 'ai' ? Boolean(generated) : Boolean(file)

  return (
    <Modal
      title="New post"
      open={open}
      onCancel={onClose}
      onOk={() => publish.mutate()}
      okText="Post"
      okButtonProps={{ disabled: !ready, loading: publish.isPending }}
      destroyOnHidden
    >
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          {
            key: 'upload',
            label: 'Upload',
            children: (
              <Upload.Dragger
                accept={demo ? 'image/jpeg,image/png,image/gif,image/webp' : 'image/jpeg,image/png,image/gif,image/webp,video/mp4,video/webm'}
                maxCount={1}
                beforeUpload={(f) => {
                  setFile(f)
                  return false // upload happens on "Post", not immediately
                }}
                onRemove={() => setFile(null)}
              >
                <p className="ant-upload-drag-icon">
                  <InboxOutlined />
                </p>
                <p className="ant-upload-text">{demo ? 'Click or drag an image' : 'Click or drag an image or video'}</p>
                <p className="ant-upload-hint">
                  {demo ? 'JPEG, PNG, GIF or WebP. Resized and kept in this browser.' : 'JPEG, PNG, GIF, WebP, MP4 or WebM, up to 50 MB'}
                </p>
              </Upload.Dragger>
            ),
          },
          {
            key: 'ai',
            label: demo ? 'Generate (demo)' : 'Generate with AI',
            disabled: !aiEnabled,
            children: (
              <Space orientation="vertical" style={{ width: '100%' }}>
                {demo && (
                  <Typography.Text type="secondary">
                    In the full app this calls OpenAI&apos;s image model from the server, with a per-user quota. The
                    demo has no key, so it draws a placeholder rain poster from your prompt.
                  </Typography.Text>
                )}
                <Input.TextArea
                  rows={3}
                  maxLength={1000}
                  placeholder="Describe the image, e.g. a corgi in a raincoat at Pike Place Market"
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                />
                <Button onClick={() => generate.mutate()} loading={generate.isPending} disabled={!prompt.trim()}>
                  Generate
                </Button>
                {generated && <Image src={generated} alt="Generated preview" width="100%" />}
              </Space>
            ),
          },
        ]}
      />
      {!aiEnabled && (
        <Typography.Text type="secondary">AI generation is off because the server has no OpenAI key.</Typography.Text>
      )}
      <Input.TextArea
        style={{ marginTop: 16 }}
        rows={2}
        maxLength={2000}
        showCount
        placeholder="Write a caption…"
        value={caption}
        onChange={(e) => setCaption(e.target.value)}
      />
    </Modal>
  )
}
