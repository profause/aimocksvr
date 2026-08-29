import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string
  kind: string
  name: string
  accountId: string
  email: string
  setToken: (token: string, kind: string, name: string) => void
  setAccountSession: (
    account: { id: string; email: string },
    token: string
  ) => void
  clearToken: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: '',
      kind: '',
      name: '',
      accountId: '',
      email: '',
      setToken: (token, kind, name) =>
        set({ token, kind, name, accountId: '', email: '' }),
      setAccountSession: (account, token) =>
        set({
          token,
          kind: 'account',
          name: '',
          accountId: account.id,
          email: account.email,
        }),
      clearToken: () =>
        set({ token: '', kind: '', name: '', accountId: '', email: '' }),
    }),
    {
      name: 'mocksvr-auth',
    }
  )
)
