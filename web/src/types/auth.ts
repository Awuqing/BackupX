export interface AuthUser {
  id: number;
  username: string;
  displayName: string;
  role: string;
}

export interface LoginPayload {
  username: string;
  password: string;
}

export interface LoginResult {
  token: string;
  user: AuthUser;
}

export type AuthStatus = 'idle' | 'bootstrapping' | 'authenticated' | 'anonymous';
