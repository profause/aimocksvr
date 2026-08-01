import { NavLink } from 'react-router'
import {
  Server,
  History,
  Upload,
  Settings,
  LayoutDashboard,
  TestTubes,
  BookOpen,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useSidebarStore } from '@/stores/sidebar-store'

const navItems = [
  { label: 'Dashboard', icon: LayoutDashboard, path: '/' },
  { label: 'Endpoints', icon: Server, path: '/endpoints' },
  { label: 'Request Logs', icon: History, path: '/logs' },
  { label: 'Scenarios', icon: TestTubes, path: '/scenarios' },
  { label: 'Imports', icon: Upload, path: '/imports' },
  { label: 'Docs', icon: BookOpen, path: '/docs' },
  { label: 'Settings', icon: Settings, path: '/settings' },
]

export function Sidebar() {
  const collapsed = useSidebarStore((state) => state.collapsed)

  return (
    <aside
      className={cn(
        'border-border bg-sidebar flex flex-col border-r transition-all duration-200',
        collapsed ? 'w-16' : 'w-56'
      )}
    >
      <nav className="flex flex-1 flex-col gap-1 p-3">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            end={item.path === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                isActive
                  ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                  : 'text-sidebar-foreground hover:bg-sidebar-accent'
              )
            }
          >
            <item.icon className="size-4 shrink-0" />
            {!collapsed && <span>{item.label}</span>}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
