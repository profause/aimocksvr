export interface ApiSuccess<T> {
  success: true
  data: T
}

export interface ApiError {
  success: false
  error: {
    code: string
    message: string
  }
}

export type ApiResponse<T> = ApiSuccess<T> | ApiError

export interface Endpoint {
  id: string
  method: string
  path: string
  description: string
  prompt: string
  response_type: string
  stateful: boolean
  status: string
  request_schema: string | null
  error_sim: string | null
  public: boolean
  version?: number
  created_at: string
  updated_at: string
}

export interface EndpointVersion {
  id: string
  endpoint_id: string
  method: string
  path: string
  description: string
  prompt: string
  response_type: string
  stateful: boolean
  status: string
  request_schema: string | null
  error_sim: string | null
  public: boolean
  schema: string | null
  version: number
  created_at: string
}

export interface RequestHistory {
  id: string
  endpoint_id: string
  request: Record<string, unknown>
  response: Record<string, unknown>
  latency: number
  created_at: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  limit: number
}

export interface CreateEndpointParams {
  method: string
  path: string
  description?: string
  prompt: string
  response_type?: string
  stateful?: boolean
  request_schema?: string
  error_sim?: string
  public?: boolean
}

export interface UpdateEndpointParams {
  description?: string
  prompt?: string
  response_type?: string
  stateful?: boolean
  request_schema?: string
  error_sim?: string
  public?: boolean
}
