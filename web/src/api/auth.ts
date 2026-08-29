import { apiClient } from './client'
import type { ApiSuccess } from '@/types/api'

interface TokenResponse {
  token: string
  kind: string
  name: string
}

export async function mintToken(apiKey: string) {
  const res = await apiClient.post<ApiSuccess<TokenResponse>>(
    '/api/v1/auth/token',
    {
      api_key: apiKey,
    }
  )
  return res.data.data
}

export interface AuthSession {
  account: {
    id: string
    email: string
  }
  token: string
}

export interface WhoamiResponse {
  kind: string
  account_id?: string
  email?: string
  name?: string
}

export async function register(email: string, password: string) {
  const res = await apiClient.post<ApiSuccess<AuthSession>>(
    '/api/v1/auth/register',
    {
      email,
      password,
    }
  )
  return res.data.data
}

export async function login(email: string, password: string) {
  const res = await apiClient.post<ApiSuccess<AuthSession>>(
    '/api/v1/auth/login',
    {
      email,
      password,
    }
  )
  return res.data.data
}

export async function whoami() {
  const res = await apiClient.get<ApiSuccess<WhoamiResponse>>(
    '/api/v1/auth/whoami'
  )
  return res.data.data
}
