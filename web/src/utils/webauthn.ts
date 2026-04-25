import type { WebAuthnAssertion, WebAuthnAttestation, WebAuthnLoginOptions, WebAuthnRegistrationOptions } from '../services/auth'

function base64UrlToBuffer(value: string) {
  const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes.buffer
}

function bufferToBase64Url(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let index = 0; index < bytes.byteLength; index += 1) {
    binary += String.fromCharCode(bytes[index])
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function assertWebAuthnAvailable() {
  if (!window.PublicKeyCredential || !navigator.credentials) {
    throw new Error('当前浏览器不支持通行密钥')
  }
}

export async function createWebAuthnCredential(options: WebAuthnRegistrationOptions): Promise<WebAuthnAttestation> {
  assertWebAuthnAvailable()
  const credential = await navigator.credentials.create({
    publicKey: {
      ...options,
      challenge: base64UrlToBuffer(options.challenge),
      user: {
        ...options.user,
        id: base64UrlToBuffer(options.user.id),
      },
      excludeCredentials: options.excludeCredentials.map((item) => ({
        ...item,
        id: base64UrlToBuffer(item.id),
      })),
    },
  }) as PublicKeyCredential | null
  if (!credential) {
    throw new Error('通行密钥创建已取消')
  }
  const response = credential.response as AuthenticatorAttestationResponse
  return {
    id: credential.id,
    rawId: bufferToBase64Url(credential.rawId),
    type: 'public-key',
    response: {
      clientDataJSON: bufferToBase64Url(response.clientDataJSON),
      attestationObject: bufferToBase64Url(response.attestationObject),
    },
  }
}

export async function getWebAuthnAssertion(options: WebAuthnLoginOptions): Promise<WebAuthnAssertion> {
  assertWebAuthnAvailable()
  const credential = await navigator.credentials.get({
    publicKey: {
      challenge: base64UrlToBuffer(options.challenge),
      rpId: options.rpId,
      timeout: options.timeout,
      userVerification: options.userVerification,
      allowCredentials: options.allowCredentials.map((item) => ({
        ...item,
        id: base64UrlToBuffer(item.id),
      })),
    },
  }) as PublicKeyCredential | null
  if (!credential) {
    throw new Error('通行密钥验证已取消')
  }
  const response = credential.response as AuthenticatorAssertionResponse
  return {
    id: credential.id,
    rawId: bufferToBase64Url(credential.rawId),
    type: 'public-key',
    response: {
      clientDataJSON: bufferToBase64Url(response.clientDataJSON),
      authenticatorData: bufferToBase64Url(response.authenticatorData),
      signature: bufferToBase64Url(response.signature),
      userHandle: response.userHandle ? bufferToBase64Url(response.userHandle) : undefined,
    },
  }
}
