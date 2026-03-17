import { ReactNode, useEffect, useRef } from 'react'
import { FullPageLoading } from './FullPageLoading'
import { useAuthStore } from '../stores/auth'

interface AuthBootstrapProps {
  children: ReactNode
}

export function AuthBootstrap({ children }: AuthBootstrapProps) {
  const bootstrap = useAuthStore((state) => state.bootstrap)
  const bootstrapped = useAuthStore((state) => state.bootstrapped)
  const startedRef = useRef(false)

  useEffect(() => {
    if (startedRef.current) {
      return
    }
    startedRef.current = true
    void bootstrap()
  }, [bootstrap])

  if (!bootstrapped) {
    return <FullPageLoading tip="正在初始化登录状态..." />
  }

  return <>{children}</>
}
