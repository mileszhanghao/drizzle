import { DeleteOutlined } from '@ant-design/icons'
import { Button, Card, Image, Popconfirm, Typography } from 'antd'
import type { Post } from '../api'

interface Props {
  post: Post
  mine: boolean
  onDelete: (id: string) => void
}

export default function PostCard({ post, mine, onDelete }: Props) {
  return (
    <Card
      size="small"
      cover={
        post.mediaType === 'video' ? (
          <video src={post.mediaUrl} controls style={{ width: '100%', aspectRatio: '1', objectFit: 'cover' }} />
        ) : (
          <Image src={post.mediaUrl} alt={post.message || `Post by ${post.username}`} style={{ aspectRatio: '1', objectFit: 'cover' }} />
        )
      }
      extra={
        mine && (
          <Popconfirm title="Delete this post?" onConfirm={() => onDelete(post.id)}>
            <Button type="text" size="small" icon={<DeleteOutlined />} aria-label="Delete post" />
          </Popconfirm>
        )
      }
      title={`@${post.username}`}
    >
      <Typography.Paragraph ellipsis={{ rows: 3, expandable: true }} style={{ marginBottom: 4 }}>
        {post.message}
      </Typography.Paragraph>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        {new Date(post.createdAt).toLocaleString()}
      </Typography.Text>
      {post.credit && (
        <Typography.Text type="secondary" className="credit">
          Photo:{' '}
          <a href={post.credit.url} target="_blank" rel="noreferrer">
            {post.credit.author}
          </a>
          , {post.credit.license}, via Wikimedia Commons
        </Typography.Text>
      )}
    </Card>
  )
}
