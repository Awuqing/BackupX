import axios from 'axios'

export interface ApiEnvelope<T> {
  code: string | number
  message: string
  data: T
}

let accessToken = ''
let unauthorizedHandler: (() => void) | null = null

export const http = axios.create({
  baseURL: '/api',
  timeout: 10000,
  withCredentials: true,
})

export function setAccessToken(token: string) {
  accessToken = token
}

export function getAccessToken() {
  return accessToken
}

export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler
}

export function unwrapApiEnvelope<T>(response: ApiEnvelope<T>) {
  return response.data
}

http.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && unauthorizedHandler) {
      unauthorizedHandler()
    }
    return Promise.reject(error)
  },
)
