import { beforeEach, describe, expect, it, vi } from 'vitest';

import { authApi } from '../services/auth';
import { useAuthStore } from './auth';

vi.mock('../services/auth', () => ({
  authApi: {
    login: vi.fn(),
    fetchProfile: vi.fn(),
  },
}));

describe('useAuthStore', () => {
  beforeEach(() => {
    window.localStorage.clear();
    useAuthStore.setState({
      token: null,
      user: null,
      hydrated: true,
      status: 'idle',
    });
    vi.clearAllMocks();
  });

  it('stores token and user after login', async () => {
    vi.mocked(authApi.login).mockResolvedValue({
      token: 'jwt-token',
      user: {
        id: 1,
        username: 'admin',
        displayName: '管理员',
        role: 'admin',
      },
    });

    await useAuthStore.getState().login({ username: 'admin', password: 'secret' });

    expect(useAuthStore.getState().token).toBe('jwt-token');
    expect(useAuthStore.getState().status).toBe('authenticated');
    expect(window.localStorage.getItem('backupx-auth-token')).toBe('jwt-token');
  });

  it('clears state when bootstrap profile request fails', async () => {
    useAuthStore.setState({ token: 'expired-token', status: 'idle' });
    vi.mocked(authApi.fetchProfile).mockRejectedValue(new Error('unauthorized'));

    await useAuthStore.getState().bootstrap();

    expect(useAuthStore.getState().token).toBeNull();
    expect(useAuthStore.getState().status).toBe('anonymous');
  });
});
