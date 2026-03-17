import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { ProtectedRoute } from './ProtectedRoute'
import { useAuthStore } from '../stores/auth'

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAuthStore.setState({
      token: '',
      user: null,
      status: 'anonymous',
      bootstrapped: true,
    })
  })

  it('redirects anonymous users to login', () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route path="/login" element={<div>login page</div>} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div>dashboard page</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('login page')).toBeInTheDocument()
  })

  it('renders protected content for authenticated users', () => {
    useAuthStore.setState({
      token: 'token',
      user: { id: 1, username: 'admin', displayName: 'Admin', role: 'admin' },
      status: 'authenticated',
      bootstrapped: true,
    })

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div>dashboard page</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('dashboard page')).toBeInTheDocument()
  })
})
