import {
  Alert,
  Button,
  Card,
  Form,
  Grid,
  Input,
  Space,
  Typography,
} from '@arco-design/web-react';
import { useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

import { useAuthStore } from '../../stores/auth';

interface LoginFormValue {
  username: string;
  password: string;
}

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useAuthStore((state) => state.login);
  const status = useAuthStore((state) => state.status);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const redirectPath = useMemo(() => {
    const from = location.state as { from?: { pathname?: string } } | null;
    return from?.from?.pathname ?? '/';
  }, [location.state]);

  async function handleSubmit(values: LoginFormValue) {
    setErrorMessage(null);

    try {
      await login(values);
      navigate(redirectPath, { replace: true });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '登录失败');
    }
  }

  return (
    <div className="fullscreen-center login-page">
      <Grid.Row justify="center" style={{ width: '100%' }}>
        <Grid.Col xs={22} sm={16} md={12} lg={8} xl={6}>
          <Card bordered={false} className="login-card">
            <Space direction="vertical" size="large" style={{ width: '100%' }}>
              <div>
                <Typography.Title heading={3}>欢迎使用 BackupX</Typography.Title>
                <Typography.Paragraph type="secondary">
                  登录后可管理备份任务、存储目标与系统状态。
                </Typography.Paragraph>
              </div>
              {errorMessage ? <Alert type="error" content={errorMessage} /> : null}
              <Form<LoginFormValue> layout="vertical" onSubmit={handleSubmit}>
                <Form.Item field="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
                  <Input autoComplete="username" placeholder="请输入管理员用户名" />
                </Form.Item>
                <Form.Item field="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
                  <Input.Password autoComplete="current-password" placeholder="请输入密码" />
                </Form.Item>
                <Button
                  long
                  type="primary"
                  htmlType="submit"
                  loading={status === 'bootstrapping'}
                >
                  登录
                </Button>
              </Form>
            </Space>
          </Card>
        </Grid.Col>
      </Grid.Row>
    </div>
  );
}
