import { Outlet } from 'react-router'

import { TopNav } from '@/layouts/top-nav'
import { Sidebar } from '@/layouts/sidebar'

export function DashboardLayout() {
  return (
    <div className="flex h-screen w-screen flex-col">
      <TopNav />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
