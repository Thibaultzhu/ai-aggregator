import { useEffect, useState } from 'react'
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard, Key, CreditCard, BarChart3, Settings,
  LogOut, Zap, ChevronLeft, User, Activity, FileText
} from 'lucide-react'
import * as api from '@/lib/api'
import type { UserProfile } from '@/lib/api'

const sidebarItems = [
  { label: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { label: 'API Keys', href: '/dashboard/keys', icon: Key },
  { label: 'Billing', href: '/dashboard/billing', icon: CreditCard },
  { label: 'Usage', href: '/dashboard', icon: BarChart3 },
  { label: 'Request Logs', href: '/dashboard/request-logs', icon: Activity },
  { label: 'Workflows', href: '/dashboard/workflows', icon: Zap },
  { label: 'Files', href: '/dashboard/files', icon: FileText },
  { label: 'Settings', href: '/dashboard/settings', icon: Settings },
]

export function DashboardLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const [profile, setProfile] = useState<UserProfile | null>(null)

  useEffect(() => {
    let cancelled = false

    api.getProfile()
      .then((res) => {
        if (!cancelled) setProfile(res)
      })
      .catch(() => {
        if (!cancelled) setProfile(null)
      })

    return () => {
      cancelled = true
    }
  }, [])

  function handleLogout() {
    api.clearAuth()
    navigate('/login')
  }

  const displayEmail = profile?.email || 'Signed in'
  const displayRole = profile?.role || 'User'

  return (
    <div className="min-h-screen flex flex-col md:flex-row">
      {/* Sidebar */}
      <aside className="w-full md:w-64 bg-gray-950 border-b md:border-b-0 md:border-r border-gray-800 flex flex-col md:fixed md:h-full z-20">
        {/* Logo */}
        <div className="p-4 border-b border-gray-800">
          <Link to="/" className="flex items-center gap-2 group">
            <div className="w-7 h-7 bg-gradient-to-br from-brand-500 to-purple-600 rounded-lg flex items-center justify-center">
              <Zap className="w-4 h-4 text-white" />
            </div>
            <span className="font-bold text-white text-sm group-hover:text-brand-400 transition-colors">
              AI Aggregator
            </span>
          </Link>
        </div>

        {/* Nav */}
        <nav className="flex-none md:flex-1 p-3 overflow-x-auto md:overflow-x-visible">
          <Link
            to="/"
            className="hidden md:flex items-center gap-2 px-3 py-2 text-xs text-gray-500 hover:text-gray-300 transition-colors"
          >
            <ChevronLeft className="w-3 h-3" /> Back to site
          </Link>
          <div className="md:pt-3">
            <p className="hidden md:block px-3 text-[11px] font-semibold text-gray-600 uppercase tracking-wider mb-2">Account</p>
            <div className="flex gap-2 md:block md:space-y-1">
            {sidebarItems.map((item) => {
              const isActive = location.pathname === item.href
              return (
                <Link
                  key={item.href + item.label}
                  to={item.href}
                  className={`flex shrink-0 items-center gap-2.5 whitespace-nowrap px-3 py-2 rounded-lg text-sm transition-colors ${
                    isActive
                      ? 'bg-brand-600/10 text-brand-400 border border-brand-500/20'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
                  }`}
                >
                  <item.icon className="w-4 h-4" />
                  {item.label}
                </Link>
              )
            })}
            </div>
          </div>
        </nav>

        {/* User */}
        <div className="hidden md:block p-3 border-t border-gray-800">
          <div className="flex items-center gap-2.5 px-3 py-2 rounded-lg bg-gray-900/40">
            <div className="w-8 h-8 bg-gray-800 rounded-full flex items-center justify-center">
              <User className="w-4 h-4 text-gray-400" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm text-gray-200 truncate" title={displayEmail}>{displayEmail}</p>
              <p className="text-xs text-gray-500 capitalize">{displayRole}</p>
            </div>
            <button
              type="button"
              onClick={handleLogout}
              className="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-800 hover:text-gray-200"
              aria-label="Log out"
              title="Log out"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex-1 min-w-0 md:ml-64">
        <Outlet />
      </div>
    </div>
  )
}
