import { Spin } from '@arco-design/web-react';
import type { PropsWithChildren } from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { useAuthStore } from '../stores/auth';

export function AuthGuard({ children }: PropsWithChildren) {
  const hydrated = useAuthStore((state) => state.hydrated);
  const status = useAuthStore((state) => state.status);
  const location = useLocation();

  if (!hydrated || status === 'bootstrapping' || status === 'idle') {
    return (
      <div className="fullscreen-center">
        <Spin tip="正在加载登录状态..." />
      </div>
    );
  }

  if (status !== 'authenticated') {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return <>{children}</>;
}
