import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { AuthGuard } from './auth-guard';
import { useAuthStore } from '../stores/auth';

function renderWithRoutes(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/login" element={<div>login-page</div>} />
        <Route
          path="/"
          element={
            <AuthGuard>
              <div>protected-page</div>
            </AuthGuard>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AuthGuard', () => {
  beforeEach(() => {
    useAuthStore.setState({
      token: null,
      user: null,
      hydrated: true,
      status: 'anonymous',
    });
  });

  it('redirects anonymous users to login page', async () => {
    renderWithRoutes('/');

    expect(await screen.findByText('login-page')).toBeInTheDocument();
  });

  it('renders children for authenticated users', async () => {
    useAuthStore.setState({
      token: 'token',
      user: {
        id: 1,
        username: 'admin',
        displayName: '管理员',
        role: 'admin',
      },
      hydrated: true,
      status: 'authenticated',
    });

    renderWithRoutes('/');

    expect(await screen.findByText('protected-page')).toBeInTheDocument();
  });
});
