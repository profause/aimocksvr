import { apiClient } from './client'
import type { ApiSuccess } from '@/types/api'

interface TokenResponse {
  token: string
  kind: string
  name: string
}

export async function mintToken(apiKey: string) {
  const res = await apiClient.post<ApiSuccess<TokenResponse>>('/api/v1/auth/token', {
    api_key: apiKey,
  })
  return res.data.data
}
