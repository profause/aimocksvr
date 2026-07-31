import { useEffect } from 'react'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type Theme = 'light' | 'dark' | 'system'

interface ThemeState {
  theme: Theme
  resolvedTheme: 'light' | 'dark'
  setTheme: (theme: Theme) => void
}

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light'
}

function applyTheme(theme: Theme): 'light' | 'dark' {
  const resolved = theme === 'system' ? getSystemTheme() : theme
  document.documentElement.classList.toggle('dark', resolved === 'dark')
  return resolved
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      theme: 'system',
      resolvedTheme: getSystemTheme(),
      setTheme: (theme) => {
        const resolved = applyTheme(theme)
        set({ theme, resolvedTheme: resolved })
      },
    }),
    {
      name: 'mocksvr-theme',
      onRehydrateStorage: () => (state) => {
        if (state) {
          applyTheme(state.theme)
        }
      },
    }
  )
)

export function useTheme() {
  const { theme, resolvedTheme, setTheme } = useThemeStore()

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (useThemeStore.getState().theme === 'system') {
        useThemeStore.setState({ resolvedTheme: getSystemTheme() })
      }
    }
    media.addEventListener('change', handler)
    return () => media.removeEventListener('change', handler)
  }, [])

  return { theme, resolvedTheme, setTheme }
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  return children
}

export { type Theme }
