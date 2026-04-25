import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { fetchProfile, login, setup, type LoginPayload, type SetupPayload, type UserInfo } from '../services/auth'
import { setAccessToken, setUnauthorizedHandler } from '../services/http'

type AuthStatus = 'unknown' | 'loading' | 'anonymous' | 'authenticated'

interface AuthState {
  token: string
  user: UserInfo | null
  status: AuthStatus
  bootstrapped: boolean
  bootstrap: () => Promise<void>
  login: (payload: LoginPayload) => Promise<void>
  setup: (payload: SetupPayload) => Promise<void>
  logout: () => void
  applyAuth: (token: string, user: UserInfo) => void
  setUser: (user: UserInfo) => void
}

function clearAuthState(set: (partial: Partial<AuthState>) => void) {
  setAccessToken('')
  set({ token: '', user: null, status: 'anonymous', bootstrapped: true })
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: '',
      user: null,
      status: 'unknown',
      bootstrapped: false,
      bootstrap: async () => {
        const token = get().token
        setUnauthorizedHandler(() => {
          clearAuthState(set)
        })

        if (!token) {
          setAccessToken('')
          set({ status: 'anonymous', bootstrapped: true })
          return
        }

        setAccessToken(token)
        set({ status: 'loading' })
        try {
          const user = await fetchProfile()
          set({ user, status: 'authenticated', bootstrapped: true })
        } catch {
          clearAuthState(set)
        }
      },
      login: async (payload) => {
        const result = await login(payload)
        get().applyAuth(result.token, result.user)
      },
      setup: async (payload) => {
        const result = await setup(payload)
        get().applyAuth(result.token, result.user)
      },
      logout: () => {
        clearAuthState(set)
      },
      applyAuth: (token, user) => {
        setAccessToken(token)
        set({ token, user, status: 'authenticated', bootstrapped: true })
      },
      setUser: (user) => {
        set({ user })
      },
    }),
    {
      name: 'backupx-auth',
      partialize: (state) => ({ token: state.token }),
    },
  ),
)
