import { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { FullPageLoading } from '../components/FullPageLoading'

interface ProtectedRouteProps {
  children: ReactNode
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const status = useAuthStore((state) => state.status)
  const bootstrapped = useAuthStore((state) => state.bootstrapped)
  const location = useLocation()

  if (!bootstrapped || status === 'loading') {
    return <FullPageLoading tip="正在验证登录状态..." />
  }

  if (status !== 'authenticated') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return <>{children}</>
}
