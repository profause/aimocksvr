import { apiClient } from './client'
import type {
  ApiSuccess,
  Endpoint,
  EndpointVersion,
  RequestHistory,
  CreateEndpointParams,
  UpdateEndpointParams,
} from '@/types/api'

interface ListEndpointsResponse {
  endpoints: Endpoint[]
  total: number
  page: number
  limit: number
}

interface ListVersionsResponse {
  versions: EndpointVersion[]
}

interface DiffResponse {
  version: number
  changes: { field: string; from: string; to: string }[]
}

interface ListHistoryResponse {
  history: RequestHistory[]
}

export async function listEndpoints(page = 1, limit = 20) {
  const res = await apiClient.get<ApiSuccess<ListEndpointsResponse>>('/api/v1/endpoints', {
    params: { page, limit },
  })
  return res.data.data
}

export async function getEndpoint(id: string) {
  const res = await apiClient.get<ApiSuccess<Endpoint>>(`/api/v1/endpoints/${id}`)
  return res.data.data
}

export async function createEndpoint(params: CreateEndpointParams) {
  const res = await apiClient.post<ApiSuccess<Endpoint>>('/api/v1/endpoints', params)
  return res.data.data
}

export async function updateEndpoint(id: string, params: UpdateEndpointParams) {
  const res = await apiClient.put<ApiSuccess<Endpoint>>(`/api/v1/endpoints/${id}`, params)
  return res.data.data
}

export async function deleteEndpoint(id: string) {
  const res = await apiClient.delete<ApiSuccess<{ id: string }>>(`/api/v1/endpoints/${id}`)
  return res.data.data
}

export async function listVersions(id: string) {
  const res = await apiClient.get<ApiSuccess<ListVersionsResponse>>(`/api/v1/endpoints/${id}/versions`)
  return res.data.data.versions
}

export async function diffVersion(id: string, version: number) {
  const res = await apiClient.get<ApiSuccess<DiffResponse>>(`/api/v1/endpoints/${id}/versions/${version}/diff`)
  return res.data.data
}

export async function rollback(id: string, version: number) {
  const res = await apiClient.post<ApiSuccess<Endpoint>>(`/api/v1/endpoints/${id}/versions/${version}/rollback`)
  return res.data.data
}

export async function listHistory(id: string) {
  const res = await apiClient.get<ApiSuccess<ListHistoryResponse>>(`/api/v1/endpoints/${id}/history`)
  return res.data.data.history
}
