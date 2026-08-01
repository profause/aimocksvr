import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface WorkspaceState {
  name: string
  apiKey: string
  setName: (name: string) => void
  setApiKey: (key: string) => void
  clearApiKey: () => void
}

export const useWorkspaceStore = create<WorkspaceState>()(
  persist(
    (set) => ({
      name: 'Default',
      apiKey: '',
      setName: (name) => set({ name }),
      setApiKey: (apiKey) => set({ apiKey }),
      clearApiKey: () => set({ apiKey: '' }),
    }),
    {
      name: 'mocksvr-workspace',
    }
  )
)
