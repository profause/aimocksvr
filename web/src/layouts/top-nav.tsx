import { Menu } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { useSidebarStore } from '@/stores/sidebar-store'
import { ThemeToggle } from '@/components/theme-toggle'

export function TopNav() {
  const toggleSidebar = useSidebarStore((state) => state.toggle)

  return (
    <header className="border-border bg-background flex h-14 items-center gap-4 border-b px-4">
      <Button
        variant="ghost"
        size="icon"
        onClick={toggleSidebar}
        className="md:hidden"
      >
        <Menu className="size-5" />
        <span className="sr-only">Toggle sidebar</span>
      </Button>
      <div className="flex items-center gap-2 font-semibold">
        <span className="text-lg">MockSvr</span>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <ThemeToggle />
      </div>
    </header>
  )
}
