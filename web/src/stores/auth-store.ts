import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string
  kind: string
  name: string
  setToken: (token: string, kind: string, name: string) => void
  clearToken: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: '',
      kind: '',
      name: '',
      setToken: (token, kind, name) => set({ token, kind, name }),
      clearToken: () => set({ token: '', kind: '', name: '' }),
    }),
    {
      name: 'mocksvr-auth',
    }
  )
)
