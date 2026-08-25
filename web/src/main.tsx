import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { Toaster } from 'sonner'

import { AppRoutes } from './routes'
import { ThemeProvider } from './stores/theme-store'
import { useAuthStore } from './stores/auth-store'
import { mintToken } from './api/auth'
import { LoadingScreen } from './components/loading-screen'

import '@/styles/globals.css'

const API_KEY = 'sk_test_abc123xyz789'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

function AuthGate({ children }: { children: React.ReactNode }) {
  const [ready, setReady] = useState(false)
  const { token, setToken } = useAuthStore()

  useEffect(() => {
    if (token) {
      setReady(true)
      return
    }

    mintToken(API_KEY)
      .then((data) => {
        setToken(data.token, data.kind, data.name)
        setReady(true)
      })
      .catch((err) => {
        console.error('Auth failed:', err)
        setReady(true)
      })
  }, [token, setToken])

  if (!ready) {
    return <LoadingScreen />
  }

  return children
}

function App() {
  return (
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <ThemeProvider>
            <AuthGate>
              <AppRoutes />
            </AuthGate>
          </ThemeProvider>
        </BrowserRouter>
        <ReactQueryDevtools initialIsOpen={false} />
        <Toaster position="top-right" />
      </QueryClientProvider>
    </StrictMode>
  )
}

createRoot(document.getElementById('root')!).render(<App />)
