import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { useAuthStore } from '../../stores/auth'
import { AdminLayout } from './AdminLayout'

describe('AdminLayout', () => {
  beforeEach(() => {
    useAuthStore.setState({
      token: 'test-token',
      user: { id: 1, username: 'admin', displayName: 'Admin', role: 'admin' },
      status: 'authenticated',
      bootstrapped: true,
    })
  })

  it('keeps user and API key management in one navigable admin area', async () => {
    const actor = userEvent.setup()
    render(
      <MemoryRouter initialEntries={['/admin/users']}>
        <Routes>
          <Route path="/admin" element={<AdminLayout />}>
            <Route path="users" element={<div>user management content</div>} />
            <Route path="api-keys" element={<div>api key management content</div>} />
          </Route>
          <Route path="/audit" element={<div>audit content</div>} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('user management content')).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: '访问管理分区' })).toBeInTheDocument()

    await actor.click(screen.getByRole('button', { name: 'API Key' }))
    expect(screen.getByText('api key management content')).toBeInTheDocument()

    await actor.click(screen.getByRole('button', { name: '访问审计' }))
    expect(screen.getByText('audit content')).toBeInTheDocument()
  })

  it('blocks non-admin users before rendering management content', () => {
    useAuthStore.setState({
      user: { id: 2, username: 'viewer', displayName: 'Viewer', role: 'viewer' },
    })

    render(
      <MemoryRouter initialEntries={['/admin/users']}>
        <Routes>
          <Route path="/admin" element={<AdminLayout />}>
            <Route path="users" element={<div>restricted content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByText('当前账号无权进入访问管理（仅管理员）')).toBeInTheDocument()
    expect(screen.queryByText('restricted content')).not.toBeInTheDocument()
  })
})
