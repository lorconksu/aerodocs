import { Outlet } from 'react-router-dom'

export function AuthLayout() {
  return (
    <div className="min-h-screen bg-base flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-xl font-bold tracking-[0.2em] text-text-primary">AERODOCS</h1>
        </div>
        <div className="bg-surface border border-border rounded-lg p-6">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
