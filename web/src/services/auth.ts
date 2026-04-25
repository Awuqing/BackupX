import { http } from './http'

export interface SetupPayload {
  username: string
  password: string
  displayName: string
}

export interface LoginPayload {
  username: string
  password: string
  twoFactorCode?: string
  webAuthnAssertion?: WebAuthnAssertion
  trustedDeviceToken?: string
  rememberDevice?: boolean
  trustedDeviceName?: string
}

export interface UserInfo {
  id: number
  username: string
  displayName: string
  email?: string
  phone?: string
  role: string
  mfaEnabled?: boolean
  twoFactorEnabled?: boolean
  twoFactorRecoveryCodesRemaining?: number
  webAuthnEnabled?: boolean
  webAuthnCredentialCount?: number
  trustedDeviceCount?: number
  emailOtpEnabled?: boolean
  smsOtpEnabled?: boolean
}

export interface AuthResult {
  token: string
  user: UserInfo
  trustedDeviceToken?: string
  trustedDevice?: TrustedDevice
}

const TRUSTED_DEVICE_TOKEN_KEY = 'backupx-trusted-device-token'
const TRUSTED_DEVICE_TOKEN_PREFIX = 'backupx-trusted-device-token:'

function trustedDeviceTokenKey(username: string) {
  return `${TRUSTED_DEVICE_TOKEN_PREFIX}${username.trim().toLowerCase()}`
}

export function getTrustedDeviceToken(username?: string) {
  if (username?.trim()) {
    return localStorage.getItem(trustedDeviceTokenKey(username)) ?? localStorage.getItem(TRUSTED_DEVICE_TOKEN_KEY) ?? ''
  }
  return localStorage.getItem(TRUSTED_DEVICE_TOKEN_KEY) ?? ''
}

export function clearTrustedDeviceToken(username?: string) {
  if (username?.trim()) {
    localStorage.removeItem(trustedDeviceTokenKey(username))
    localStorage.removeItem(TRUSTED_DEVICE_TOKEN_KEY)
    return
  }
  localStorage.removeItem(TRUSTED_DEVICE_TOKEN_KEY)
  for (let index = localStorage.length - 1; index >= 0; index -= 1) {
    const key = localStorage.key(index)
    if (key?.startsWith(TRUSTED_DEVICE_TOKEN_PREFIX)) {
      localStorage.removeItem(key)
    }
  }
}

export async function fetchSetupStatus() {
  const response = await http.get<{ code: string; message: string; data: { initialized: boolean } }>('/auth/setup/status')
  return response.data.data
}

export async function setup(payload: SetupPayload) {
  const response = await http.post<{ code: string; message: string; data: AuthResult }>('/auth/setup', payload)
  return response.data.data
}

export async function login(payload: LoginPayload) {
  const response = await http.post<{ code: string; message: string; data: AuthResult }>('/auth/login', {
    ...payload,
    trustedDeviceToken: payload.trustedDeviceToken ?? getTrustedDeviceToken(payload.username),
  })
  const result = response.data.data
  if (result.trustedDeviceToken) {
    localStorage.setItem(trustedDeviceTokenKey(payload.username), result.trustedDeviceToken)
    localStorage.removeItem(TRUSTED_DEVICE_TOKEN_KEY)
  }
  return result
}

export async function fetchProfile() {
  const response = await http.get<{ code: string; message: string; data: UserInfo }>('/auth/profile')
  return response.data.data
}

export interface ChangePasswordPayload {
  oldPassword: string
  newPassword: string
}

export async function changePassword(payload: ChangePasswordPayload) {
  const response = await http.put<{ code: string; message: string; data: { changed: boolean } }>('/auth/password', payload)
  return response.data.data
}

export interface TwoFactorSetupPayload {
  currentPassword: string
}

export interface TwoFactorSetupResult {
  secret: string
  otpAuthUrl: string
  qrCodeDataUrl: string
  twoFactorEnabled: boolean
  twoFactorConfirmed: boolean
}

export interface TwoFactorCodesResult {
  user: UserInfo
  recoveryCodes: string[]
}

export interface EnableTwoFactorPayload {
  code: string
}

export interface DisableTwoFactorPayload {
  currentPassword: string
  code: string
}

export type RegenerateRecoveryCodesPayload = DisableTwoFactorPayload

export type OTPChannel = 'email' | 'sms'

export interface OTPConfigPayload {
  currentPassword: string
  channel: OTPChannel
  enabled: boolean
  email?: string
  phone?: string
}

export interface SendLoginOTPPayload {
  username: string
  password: string
  channel: OTPChannel
}

export interface WebAuthnCredentialDescriptor {
  type: 'public-key'
  id: string
}

export interface WebAuthnRegistrationOptions {
  challenge: string
  rp: { name: string; id: string }
  user: { id: string; name: string; displayName: string }
  pubKeyCredParams: Array<{ type: 'public-key'; alg: number }>
  timeout: number
  attestation: 'none'
  authenticatorSelection: { userVerification: UserVerificationRequirement }
  excludeCredentials: WebAuthnCredentialDescriptor[]
}

export interface WebAuthnLoginOptions {
  challenge: string
  rpId: string
  timeout: number
  userVerification: UserVerificationRequirement
  allowCredentials: WebAuthnCredentialDescriptor[]
}

export interface WebAuthnAttestation {
  id: string
  rawId: string
  type: 'public-key'
  response: {
    clientDataJSON: string
    attestationObject: string
  }
}

export interface WebAuthnAssertion {
  id: string
  rawId: string
  type: 'public-key'
  response: {
    clientDataJSON: string
    authenticatorData: string
    signature: string
    userHandle?: string
  }
}

export interface WebAuthnCredential {
  id: string
  name: string
  createdAt: string
  lastUsedAt?: string
}

export interface TrustedDevice {
  id: string
  name: string
  createdAt: string
  lastUsedAt: string
  expiresAt: string
  lastIp: string
}

export async function prepareTwoFactor(payload: TwoFactorSetupPayload) {
  const response = await http.post<{ code: string; message: string; data: TwoFactorSetupResult }>('/auth/2fa/setup', payload)
  return response.data.data
}

export async function enableTwoFactor(payload: EnableTwoFactorPayload) {
  const response = await http.post<{ code: string; message: string; data: TwoFactorCodesResult }>('/auth/2fa/enable', payload)
  return response.data.data
}

export async function regenerateRecoveryCodes(payload: RegenerateRecoveryCodesPayload) {
  const response = await http.post<{ code: string; message: string; data: TwoFactorCodesResult }>('/auth/2fa/recovery-codes', payload)
  return response.data.data
}

export async function disableTwoFactor(payload: DisableTwoFactorPayload) {
  const response = await http.delete<{ code: string; message: string; data: UserInfo }>('/auth/2fa', { data: payload })
  return response.data.data
}

export async function configureOtp(payload: OTPConfigPayload) {
  const response = await http.put<{ code: string; message: string; data: UserInfo }>('/auth/otp/config', payload)
  return response.data.data
}

export async function sendLoginOtp(payload: SendLoginOTPPayload) {
  const response = await http.post<{ code: string; message: string; data: { sent: boolean } }>('/auth/otp/send', payload)
  return response.data.data
}

export async function beginWebAuthnRegistration(payload: { currentPassword: string }) {
  const response = await http.post<{ code: string; message: string; data: WebAuthnRegistrationOptions }>('/auth/webauthn/register/options', payload)
  return response.data.data
}

export async function finishWebAuthnRegistration(payload: { name?: string; credential: WebAuthnAttestation }) {
  const response = await http.post<{ code: string; message: string; data: UserInfo }>('/auth/webauthn/register/finish', payload)
  return response.data.data
}

export async function beginWebAuthnLogin(payload: { username: string; password: string }) {
  const response = await http.post<{ code: string; message: string; data: WebAuthnLoginOptions }>('/auth/webauthn/login/options', payload)
  return response.data.data
}

export async function listWebAuthnCredentials() {
  const response = await http.get<{ code: string; message: string; data: WebAuthnCredential[] }>('/auth/webauthn/credentials')
  return response.data.data
}

export async function deleteWebAuthnCredential(id: string, payload: { currentPassword: string }) {
  const response = await http.delete<{ code: string; message: string; data: UserInfo }>(`/auth/webauthn/credentials/${id}`, { data: payload })
  return response.data.data
}

export async function listTrustedDevices() {
  const response = await http.get<{ code: string; message: string; data: TrustedDevice[] }>('/auth/trusted-devices')
  return response.data.data
}

export async function revokeTrustedDevice(id: string, payload: { currentPassword: string }) {
  const response = await http.delete<{ code: string; message: string; data: { deleted: boolean } }>(`/auth/trusted-devices/${id}`, { data: payload })
  return response.data.data
}

export async function logout() {
  const response = await http.post<{ code: string; message: string; data: { loggedOut: boolean } }>('/auth/logout')
  return response.data.data
}
