import { Outlet } from 'react-router-dom'
import { Logo } from '@/components/logo'

export function AuthLayout() {
  return (
    <div className="min-h-screen bg-base flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="flex justify-center mb-8">
          <Logo layout="vertical" className="w-20" />
        </div>
        <div className="bg-surface border border-border rounded-lg p-6">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
