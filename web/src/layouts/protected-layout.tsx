import {
  Button,
  Layout,
  Menu,
  Space,
  Typography,
} from '@arco-design/web-react';
import {
  IconDashboard,
  IconInfoCircle,
  IconPoweroff,
} from '@arco-design/web-react/icon';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useAuthStore } from '../stores/auth';

const { Sider, Header, Content } = Layout;

const menuItems = [
  {
    key: '/',
    label: '仪表盘',
    icon: <IconDashboard />,
  },
  {
    key: '/system-info',
    label: '系统信息',
    icon: <IconInfoCircle />,
  },
];

export function ProtectedLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible breakpoint="lg" className="app-sider">
        <div className="app-logo">BackupX</div>
        <Menu
          selectedKeys={[location.pathname]}
          onClickMenuItem={(key) => {
            navigate(key);
          }}
          style={{ width: '100%' }}
        >
          {menuItems.map((item) => (
            <Menu.Item key={item.key} data-testid={`menu-${item.key}`}>
              {item.icon}
              {item.label}
            </Menu.Item>
          ))}
        </Menu>
      </Sider>
      <Layout>
        <Header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 20px', background: 'var(--color-bg-2)', borderBottom: '1px solid var(--color-border)' }}>
          <Space size="large">
            <Typography.Title heading={6} style={{ margin: 0 }}>
              BackupX 管理台
            </Typography.Title>
            <Typography.Text type="secondary">
              {user?.displayName ?? user?.username ?? '未登录'}
            </Typography.Text>
            <Button
              icon={<IconPoweroff />}
              type="outline"
              onClick={() => {
                logout();
                navigate('/login', { replace: true });
              }}
            >
              退出登录
            </Button>
          </Space>
        </Header>
        <Content style={{ padding: '24px', background: 'var(--color-bg-1)', overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
