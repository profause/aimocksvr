import { Moon, Sun, Monitor } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { useThemeStore, type Theme } from '@/stores/theme-store'

const themes: { value: Theme; icon: typeof Sun; label: string }[] = [
  { value: 'light', icon: Sun, label: 'Light' },
  { value: 'dark', icon: Moon, label: 'Dark' },
  { value: 'system', icon: Monitor, label: 'System' },
]

export function ThemeToggle() {
  const { theme, setTheme } = useThemeStore()
  const nextTheme = (): Theme => {
    const idx = themes.findIndex((t) => t.value === theme)
    return themes[(idx + 1) % themes.length].value
  }

  const Icon = themes.find((t) => t.value === theme)?.icon ?? Sun

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={() => setTheme(nextTheme())}
      title={`Theme: ${theme}`}
    >
      <Icon className="size-4" />
      <span className="sr-only">Toggle theme</span>
    </Button>
  )
}
