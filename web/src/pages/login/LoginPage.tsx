import { Alert, Button, Card, Form, Input, Space, Typography, Message } from '@arco-design/web-react'
import { IconCloud, IconLock, IconUser } from '@arco-design/web-react/icon'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import { fetchSetupStatus } from '../../services/auth'
import { useAuthStore } from '../../stores/auth'

interface SetupFormValues {
  username: string
  password: string
  displayName: string
}

interface LoginFormValues {
  username: string
  password: string
}

function resolveErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    return error.response?.data?.message ?? '请求失败，请稍后重试'
  }
  return '请求失败，请稍后重试'
}

export function LoginPage() {
  const navigate = useNavigate()
  const authStatus = useAuthStore((state) => state.status)
  const doLogin = useAuthStore((state) => state.login)
  const doSetup = useAuthStore((state) => state.setup)
  const [initialized, setInitialized] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (authStatus === 'authenticated') {
      navigate('/dashboard', { replace: true })
    }
  }, [authStatus, navigate])

  useEffect(() => {
    let mounted = true
    void (async () => {
      try {
        const result = await fetchSetupStatus()
        if (mounted) {
          setInitialized(result.initialized)
        }
      } catch {
        if (mounted) {
          setInitialized(true)
        }
      }
    })()
    return () => {
      mounted = false
    }
  }, [])

  const handleSetup = async (values: SetupFormValues) => {
    setLoading(true)
    try {
      await doSetup(values)
      Message.success('初始化完成，正在进入控制台')
      navigate('/dashboard', { replace: true })
    } catch (error) {
      Message.error(resolveErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  const handleLogin = async (values: LoginFormValues) => {
    setLoading(true)
    try {
      await doLogin(values)
      Message.success('登录成功')
      navigate('/dashboard', { replace: true })
    } catch (error) {
      Message.error(resolveErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-shell">
      <div className="login-bg" />
      <div className="login-container">
        <div className="login-banner">
          {/* Background decorative circles for the banner */}
          <div style={{ position: 'absolute', width: 400, height: 400, borderRadius: '50%', background: 'rgba(255,255,255,0.05)', top: -100, right: -100 }} />
          <div style={{ position: 'absolute', width: 300, height: 300, borderRadius: '50%', background: 'rgba(255,255,255,0.05)', bottom: -50, left: -50 }} />
          
          <div className="login-banner-inner">
            <svg width="320" height="320" viewBox="0 0 320 320" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ marginBottom: 16 }}>
              {/* Outer pulsing rings */}
              <circle cx="160" cy="160" r="120" fill="white" fillOpacity="0.05">
                <animate attributeName="r" values="115;125;115" dur="4s" repeatCount="indefinite"/>
                <animate attributeName="fill-opacity" values="0.03;0.08;0.03" dur="4s" repeatCount="indefinite"/>
              </circle>
              <circle cx="160" cy="160" r="80" fill="white" fillOpacity="0.1">
                <animate attributeName="r" values="75;85;75" dur="3s" repeatCount="indefinite"/>
              </circle>
              
              <g>
                <animateTransform attributeName="transform" type="translate" values="0,0; 0,-8; 0,0" dur="5s" repeatCount="indefinite"/>
                {/* Layer 1 (Top) */}
                <path d="M120 120C120 111.163 137.909 104 160 104C182.091 104 200 111.163 200 120V144C200 152.837 182.091 160 160 160C137.909 160 120 152.837 120 144V120Z" fill="white" fillOpacity="0.95"/>
                <ellipse cx="160" cy="120" rx="40" ry="16" fill="white"/>
                
                {/* Layer 2 (Middle) */}
                <path d="M120 152C120 143.163 137.909 136 160 136C182.091 136 200 143.163 200 152V176C200 184.837 182.091 192 160 192C137.909 192 120 184.837 120 176V152Z" fill="white" fillOpacity="0.75"/>
                <ellipse cx="160" cy="152" rx="40" ry="16" fill="white" fillOpacity="0.9"/>

                {/* Layer 3 (Bottom) */}
                <path d="M120 184C120 175.163 137.909 168 160 168C182.091 168 200 175.163 200 184V208C200 216.837 182.091 224 160 224C137.909 224 120 216.837 120 208V184Z" fill="white" fillOpacity="0.5"/>
                <ellipse cx="160" cy="184" rx="40" ry="16" fill="white" fillOpacity="0.6"/>
                
                {/* Glowing Dots Output - Animated */}
                <g fill="var(--color-primary-6, #165dff)">
                  <circle cx="140" cy="120" r="4">
                    <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="0s" repeatCount="indefinite"/>
                  </circle>
                  <circle cx="140" cy="152" r="4">
                    <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="0.6s" repeatCount="indefinite"/>
                  </circle>
                  <circle cx="140" cy="184" r="4">
                    <animate attributeName="opacity" values="0.3;1;0.3" dur="2s" begin="1.2s" repeatCount="indefinite"/>
                  </circle>
                </g>
                
                {/* Connecting Data Line */}
                <path d="M160 120V152V184" stroke="var(--color-primary-6, #165dff)" strokeWidth="2" strokeDasharray="4 4" opacity="0.6">
                  <animate attributeName="stroke-dashoffset" from="16" to="0" dur="1s" repeatCount="indefinite" />
                </path>
              </g>
            </svg>

            <Typography.Title heading={2} style={{ color: 'white', marginTop: 0, marginBottom: 12, fontWeight: 700 }}>
              守护您的数据资产
            </Typography.Title>
            <Typography.Text style={{ color: 'rgba(255,255,255,0.75)', fontSize: 16 }}>
              安全、可靠、高效的企业级服务器备份管理平台
            </Typography.Text>
          </div>
        </div>
        
        <div className="login-form-wrapper">
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <div style={{ paddingBottom: 8 }}>
              <div style={{ display: 'inline-flex', alignItems: 'center', marginBottom: 16 }}>
                <div style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: 36, height: 36, borderRadius: 10, background: 'linear-gradient(135deg, var(--color-primary-5) 0%, var(--color-primary-7) 100%)', marginRight: 12 }}>
                  <IconCloud style={{ fontSize: 20, color: 'white' }} />
                </div>
                <Typography.Title heading={4} style={{ margin: 0, fontWeight: 700 }}>
                  BackupX
                </Typography.Title>
              </div>
              <Typography.Title heading={3} style={{ marginTop: 0, marginBottom: 8, fontWeight: 600 }}>
                {initialized === false ? '系统初始化' : '欢迎回来'}
              </Typography.Title>
              <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 14 }}>
                {initialized === false ? '请设定首个管理员账户以启动系统。' : '请输入管理员账户信息登录控制台。'}
              </Typography.Paragraph>
            </div>

            {initialized === false ? (
              <Form<SetupFormValues> layout="vertical" onSubmit={handleSetup}>
                <Form.Item field="displayName" label="显示名称" rules={[{ required: true, minLength: 1 }]}>
                  <Input placeholder="请输入显示名称" prefix={<IconUser />} size="large" />
                </Form.Item>
                <Form.Item field="username" label="用户名" rules={[{ required: true, minLength: 3 }]}>
                  <Input placeholder="请输入管理员用户名" prefix={<IconUser />} size="large" />
                </Form.Item>
                <Form.Item field="password" label="密码" rules={[{ required: true, minLength: 8 }]}>
                  <Input.Password placeholder="请输入至少 8 位密码" prefix={<IconLock />} size="large" />
                </Form.Item>
                <Button long type="primary" htmlType="submit" loading={loading} size="large" style={{ borderRadius: 8, height: 44, marginTop: 8 }}>
                  初始化并登录
                </Button>
              </Form>
            ) : (
              <Form<LoginFormValues> layout="vertical" onSubmit={handleLogin}>
                <Form.Item field="username" label="用户名" rules={[{ required: true, minLength: 3 }]}>
                  <Input placeholder="请输入用户名" prefix={<IconUser />} size="large" />
                </Form.Item>
                <Form.Item field="password" label="密码" rules={[{ required: true, minLength: 8 }]}>
                  <Input.Password placeholder="请输入密码" prefix={<IconLock />} size="large" />
                </Form.Item>
                <Button long type="primary" htmlType="submit" loading={loading} size="large" style={{ borderRadius: 8, height: 44, marginTop: 16 }}>
                  登录
                </Button>
              </Form>
            )}
          </Space>
        </div>
      </div>
    </div>
  )
}
