import { LogOut, Menu } from 'lucide-react'
import { Link, useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import { useSidebarStore } from '@/stores/sidebar-store'
import { useAuthStore } from '@/stores/auth-store'
import { ThemeToggle } from '@/components/theme-toggle'

export function TopNav() {
  const toggleSidebar = useSidebarStore((state) => state.toggle)
  const token = useAuthStore((state) => state.token)
  const email = useAuthStore((state) => state.email)
  const clearToken = useAuthStore((state) => state.clearToken)
  const navigate = useNavigate()

  function handleLogout() {
    clearToken()
    navigate('/login', { replace: true })
  }

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
        {token ? (
          <>
            {email && (
              <span className="text-muted-foreground text-sm">{email}</span>
            )}
            <Button variant="ghost" onClick={handleLogout}>
              <LogOut />
              Logout
            </Button>
          </>
        ) : (
          <>
            <Button variant="ghost" asChild>
              <Link to="/login">Sign in</Link>
            </Button>
            <Button asChild>
              <Link to="/signup">Sign up</Link>
            </Button>
          </>
        )}
        <ThemeToggle />
      </div>
    </header>
  )
}
