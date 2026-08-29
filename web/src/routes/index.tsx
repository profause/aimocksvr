import { Navigate, Route, Routes } from 'react-router'

import { useAuthStore } from '@/stores/auth-store'
import { DashboardLayout } from '@/layouts/dashboard-layout'
import { HomePage } from '@/pages/home-page'
import { EndpointsPage } from '@/pages/endpoints-page'
import { LogsPage } from '@/pages/logs-page'
import { ScenariosPage } from '@/pages/scenarios-page'
import { ImportsPage } from '@/pages/imports-page'
import { DocsPage } from '@/pages/docs-page'
import { SettingsPage } from '@/pages/settings-page'
import { LoginPage } from '@/pages/login-page'
import { SignupPage } from '@/pages/signup-page'
import { NotFound } from '@/pages/not-found-page'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return children
}

function GuestOnly({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (token) {
    return <Navigate to="/" replace />
  }
  return children
}

export function AppRoutes() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <GuestOnly>
            <LoginPage />
          </GuestOnly>
        }
      />
      <Route
        path="/signup"
        element={
          <GuestOnly>
            <SignupPage />
          </GuestOnly>
        }
      />
      <Route
        element={
          <RequireAuth>
            <DashboardLayout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<HomePage />} />
        <Route path="/endpoints" element={<EndpointsPage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/scenarios" element={<ScenariosPage />} />
        <Route path="/imports" element={<ImportsPage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}
