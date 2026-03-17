import { Space, Spin, Typography } from '@arco-design/web-react'

interface FullPageLoadingProps {
  tip: string
}

export function FullPageLoading({ tip }: FullPageLoadingProps) {
  return (
    <div className="full-page-shell">
      <Space direction="vertical" size="large" align="center">
        <Spin size={32} />
        <Typography.Text>{tip}</Typography.Text>
      </Space>
    </div>
  )
}
