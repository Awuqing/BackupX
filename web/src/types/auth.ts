export interface AuthUser {
  id: number;
  username: string;
  displayName: string;
  email?: string;
  phone?: string;
  role: string;
  mfaEnabled?: boolean;
  twoFactorEnabled?: boolean;
  twoFactorRecoveryCodesRemaining?: number;
  webAuthnEnabled?: boolean;
  webAuthnCredentialCount?: number;
  trustedDeviceCount?: number;
  emailOtpEnabled?: boolean;
  smsOtpEnabled?: boolean;
}

export interface LoginPayload {
  username: string;
  password: string;
  twoFactorCode?: string;
  webAuthnAssertion?: unknown;
  trustedDeviceToken?: string;
  rememberDevice?: boolean;
  trustedDeviceName?: string;
}

export interface LoginResult {
  token: string;
  user: AuthUser;
}

export type AuthStatus = 'idle' | 'bootstrapping' | 'authenticated' | 'anonymous';
