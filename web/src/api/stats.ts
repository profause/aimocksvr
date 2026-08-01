import { apiClient } from './client'
import type { ApiSuccess } from '@/types/api'

export interface DashboardStats {
  total_endpoints: number
  active_requests: number
  avg_latency: number
  error_rate: number
}

export async function getDashboardStats() {
  const res = await apiClient.get<ApiSuccess<DashboardStats>>('/api/v1/stats')
  return res.data.data
}
