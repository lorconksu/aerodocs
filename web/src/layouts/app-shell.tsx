import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { LayoutDashboard, ScrollText, Settings, LogOut } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '@/hooks/use-auth'
import { Logo } from '@/components/logo'
import { apiFetch } from '@/lib/api'
import type { ServerListResponse } from '@/types/api'

export function AppShell() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const { data: serverData } = useQuery({
    queryKey: ['servers'],
    queryFn: () => apiFetch<ServerListResponse>('/servers?limit=1000'),
  })

  const servers = serverData?.servers ?? []
  const onlineCount = servers.filter((s) => s.status === 'online').length
  const offlineCount = servers.filter((s) => s.status === 'offline').length
  const pendingCount = servers.filter((s) => s.status === 'pending').length

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { to: '/', icon: LayoutDashboard, label: 'Fleet Dashboard' },
    { to: '/audit-logs', icon: ScrollText, label: 'Audit Logs' },
    { to: '/settings', icon: Settings, label: 'Settings' },
  ]

  return (
    <div className="min-h-screen bg-base flex flex-col">
      {/* Top Telemetry Bar */}
      <header className="bg-surface border-b border-border px-4 py-2 flex items-center justify-between text-xs">
        <div className="flex items-center gap-4">
          <Logo layout="horizontal" className="w-28" />
          <span className="text-text-faint">|</span>
          <span className="text-text-muted uppercase tracking-widest text-[10px]">Fleet Health</span>
          <span className="text-status-online">● {onlineCount} Online</span>
          <span className="text-status-offline">● {offlineCount} Offline</span>
          {pendingCount > 0 && (
            <span className="text-status-warning">● {pendingCount} Pending</span>
          )}
        </div>
        <div className="flex items-center gap-3">
          <span className="text-text-secondary">{user?.username}</span>
          <button
            onClick={handleLogout}
            className="text-text-muted hover:text-text-primary transition-colors"
            title="Sign out"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </header>

      <div className="flex flex-1">
        {/* Left Sidebar */}
        <nav className="w-52 bg-surface/50 border-r border-border flex flex-col py-3">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-4 py-2 text-sm transition-colors ${
                  isActive
                    ? 'text-text-primary bg-elevated border-l-2 border-accent'
                    : 'text-text-muted hover:text-text-secondary border-l-2 border-transparent'
                }`
              }
            >
              <Icon className="w-4 h-4" />
              {label}
            </NavLink>
          ))}
          <div className="flex-1" />
          <div className="px-4 text-[10px] text-text-faint uppercase tracking-widest">v0.1.0</div>
        </nav>

        {/* Main Content */}
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
