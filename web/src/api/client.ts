import axios from 'axios'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const message =
      error.response?.data?.error?.message ??
      error.response?.data?.message ??
      error.message ??
      'An unexpected error occurred'
    return Promise.reject({
      code: error.response?.data?.error?.code ?? 'UNKNOWN_ERROR',
      message,
      status: error.response?.status,
    })
  }
)

export { apiClient }
