import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/lib/query-client'
import { AuthProvider, useAuth } from '@/hooks/use-auth'
import { AuthLayout } from '@/layouts/auth-layout'
import { AppShell } from '@/layouts/app-shell'
import { LoginPage } from '@/pages/login'
import { LoginTOTPPage } from '@/pages/login-totp'
import { SetupPage } from '@/pages/setup'
import { SetupTOTPPage } from '@/pages/setup-totp'
import { DashboardPage } from '@/pages/dashboard'
import { AuditLogsPage } from '@/pages/audit-logs'
import { SettingsPage } from '@/pages/settings'
import { ServerDetailPage } from '@/pages/server-detail'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-base flex items-center justify-center">
        <div className="text-text-muted text-sm">Loading...</div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

function AppRoutes() {
  return (
    <Routes>
      {/* Public auth routes */}
      <Route element={<AuthLayout />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/login/totp" element={<LoginTOTPPage />} />
        <Route path="/setup" element={<SetupPage />} />
        <Route path="/setup/totp" element={<SetupTOTPPage />} />
      </Route>

      {/* Protected routes */}
      <Route element={<ProtectedRoute><AppShell /></ProtectedRoute>}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/audit-logs" element={<AuditLogsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/servers/:id" element={<ServerDetailPage />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
