import { Routes, Route } from 'react-router'

import { DashboardLayout } from '@/layouts/dashboard-layout'
import { HomePage } from '@/pages/home-page'
import { EndpointsPage } from '@/pages/endpoints-page'
import { LogsPage } from '@/pages/logs-page'
import { ScenariosPage } from '@/pages/scenarios-page'
import { ImportsPage } from '@/pages/imports-page'
import { DocsPage } from '@/pages/docs-page'
import { SettingsPage } from '@/pages/settings-page'

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<DashboardLayout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/endpoints" element={<EndpointsPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/scenarios" element={<ScenariosPage />} />
        <Route path="/imports" element={<ImportsPage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  )
}
