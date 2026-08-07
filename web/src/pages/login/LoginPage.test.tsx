import { act, cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { setApplicationLanguage } from '../../i18n'
import { LoginPage } from './LoginPage'

const mocks = vi.hoisted(() => ({
  fetchSetupStatus: vi.fn(),
  login: vi.fn(),
  setup: vi.fn(),
}))

vi.mock('../../services/auth', () => ({
  beginWebAuthnLogin: vi.fn(),
  fetchSetupStatus: mocks.fetchSetupStatus,
  sendLoginOtp: vi.fn(),
}))

vi.mock('../../stores/auth', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => selector({
    status: 'anonymous',
    login: mocks.login,
    setup: mocks.setup,
  }),
}))

vi.mock('../../utils/webauthn', () => ({
  getWebAuthnAssertion: vi.fn(),
}))

describe('LoginPage initialization', () => {
  beforeEach(async () => {
    mocks.fetchSetupStatus.mockReset()
    mocks.login.mockReset()
    mocks.setup.mockReset()
    await act(() => setApplicationLanguage('en-US'))
  })

  afterEach(async () => {
    cleanup()
    await act(() => setApplicationLanguage('zh-CN'))
  })

  it('shows the first-administrator form in English', async () => {
    mocks.fetchSetupStatus.mockResolvedValue({ initialized: false })

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('System setup')).toBeInTheDocument()
    expect(screen.getByText('Create the first administrator account.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create administrator and sign in' })).toBeInTheDocument()
  })

  it('does not mistake an unreachable fresh install for an initialized system', async () => {
    mocks.fetchSetupStatus.mockRejectedValue(new Error('connection refused'))

    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Unable to check initialization status')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(screen.queryByText('Welcome back')).not.toBeInTheDocument()
  })
})
