import { Alert, Button, Card, Space, Spin, Typography } from '@arco-design/web-react'
import axios from 'axios'
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { completeGoogleDriveAuth } from '../../services/storage-targets'
import type { GoogleDriveCallbackResult } from '../../types/storage-targets'

function resolveErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    return error.response?.data?.message ?? 'Google Drive 授权回调失败'
  }
  return 'Google Drive 授权回调失败'
}

// Define outside the component to survive React StrictMode unmount/remount
let globalAuthPromise: Promise<GoogleDriveCallbackResult> | null = null

export function GoogleDriveCallbackPage() {
  const [searchParams] = useSearchParams()
  const [loading, setLoading] = useState(true)
  const [result, setResult] = useState<GoogleDriveCallbackResult | null>(null)
  const [error, setError] = useState('')
  const [countdown, setCountdown] = useState(3)

  useEffect(() => {
    let active = true

    if (!globalAuthPromise) {
      globalAuthPromise = completeGoogleDriveAuth(searchParams.toString())
    }

    globalAuthPromise
      .then((response) => {
        if (active) setResult(response)
      })
      .catch((callbackError) => {
        if (active) setError(resolveErrorMessage(callbackError))
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [searchParams])

  // Auto-close countdown on success
  useEffect(() => {
    if (!result?.success) return
    if (countdown <= 0) {
      window.close()
      return
    }
    const timer = setTimeout(() => setCountdown((c) => c - 1), 1000)
    return () => clearTimeout(timer)
  }, [result, countdown])

  function handleClose() {
    window.close()
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', padding: 24 }}>
      <Card style={{ maxWidth: 520, width: '100%' }}>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <div>
            <Typography.Title heading={4}>Google Drive 授权结果</Typography.Title>
            <Typography.Paragraph type="secondary">
              BackupX 正在处理 Google Drive OAuth 回调结果。
            </Typography.Paragraph>
          </div>

          {loading ? <Spin tip="正在完成授权..." style={{ width: '100%' }} /> : null}

          {!loading && error ? <Alert type="error" content={error} /> : null}

          {!loading && !error && result ? (
            <Alert
              type={result.success ? 'success' : 'warning'}
              content={
                result.success
                  ? `${result.message}，此页面将在 ${countdown} 秒后自动关闭...`
                  : result.message
              }
            />
          ) : null}

          <Space>
            {!loading && result?.success ? (
              <Button type="primary" onClick={handleClose}>
                立即关闭此页面
              </Button>
            ) : null}
            {!loading && (error || !result?.success) ? (
              <Button type="primary" onClick={handleClose}>
                关闭页面
              </Button>
            ) : null}
          </Space>
        </Space>
      </Card>
    </div>
  )
}

