import { apiClient } from './client'
import type { ApiSuccess } from '@/types/api'

interface ImportResult {
  parsed: number
  created: number
  skipped: number
  endpoints: { id: string; method: string; path: string }[]
}

export async function importOpenAPI(file: File) {
  const formData = new FormData()
  formData.append('file', file)

  const res = await apiClient.post<ApiSuccess<ImportResult>>('/api/v1/imports/openapi', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.data
}

export async function importPostman(file: File) {
  const formData = new FormData()
  formData.append('file', file)

  const res = await apiClient.post<ApiSuccess<ImportResult>>('/api/v1/imports/postman', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.data
}
