import { useState } from 'react'
import { Routes, Route, Link, useLocation } from 'react-router-dom'
import {
  LayoutDashboard, Box, Users, Key, BarChart3, Settings, Bell, FileText,
  Zap, TrendingUp, AlertTriangle, CheckCircle, XCircle
} from 'lucide-react'
import { cn } from '@/lib/utils'

const adminNav = [
  { label: 'Overview', path: '/admin', icon: LayoutDashboard },
  { label: 'Models', path: '/admin/models', icon: Box },
  { label: 'Users', path: '/admin/users', icon: Users },
  { label: 'API Keys', path: '/admin/keys', icon: Key },
  { label: 'Analytics', path: '/admin/analytics', icon: BarChart3 },
  { label: 'Alerts', path: '/admin/alerts', icon: Bell },
  { label: 'Settings', path: '/admin/settings', icon: Settings },
  { label: 'Audit Log', path: '/admin/audit', icon: FileText },
]

export default function Admin() {
  return (
    <div className="min-h-screen flex bg-gray-950">
      {/* Sidebar */}
      <aside className="w-56 bg-gray-950 border-r border-gray-800 p-4 fixed h-full">
        <Link to="/" className="flex items-center gap-2 mb-8 px-2">
          <div className="w-7 h-7 bg-gradient-to-br from-brand-500 to-purple-600 rounded-lg flex items-center justify-center">
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="font-bold text-white text-sm">Admin Panel</span>
        </Link>
        <nav className="space-y-1">
          {adminNav.map(({ label, path, icon: Icon }) => (
            <AdminNavLink key={path} label={label} path={path} icon={Icon} />
          ))}
        </nav>
      </aside>

      {/* Content */}
      <div className="flex-1 ml-56 p-8">
        <Routes>
          <Route index element={<AdminOverview />} />
          <Route path="models" element={<AdminModels />} />
          <Route path="users" element={<div className="text-gray-400">Users management — TODO</div>} />
          <Route path="keys" element={<div className="text-gray-400">API Keys management — TODO</div>} />
          <Route path="analytics" element={<div className="text-gray-400">Analytics dashboard — TODO</div>} />
          <Route path="alerts" element={<div className="text-gray-400">Alert rules — TODO</div>} />
          <Route path="settings" element={<div className="text-gray-400">System settings — TODO</div>} />
          <Route path="audit" element={<div className="text-gray-400">Audit log — TODO</div>} />
        </Routes>
      </div>
    </div>
  )
}

function AdminNavLink({ label, path, icon: Icon }: { label: string; path: string; icon: any }) {
  const location = useLocation()
  const isActive = location.pathname === path

  return (
    <Link
      to={path}
      className={cn(
        'flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors',
        isActive
          ? 'bg-brand-600/10 text-brand-400 border border-brand-500/20'
          : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
      )}
    >
      <Icon className="w-4 h-4" /> {label}
    </Link>
  )
}

// ===== Admin Overview =====

function AdminOverview() {
  const stats = [
    { label: 'Total Requests (24h)', value: '45,892', change: '+18%', positive: true },
    { label: 'Active Users', value: '1,247', change: '+5%', positive: true },
    { label: 'Error Rate', value: '0.8%', change: '-0.2%', positive: true },
    { label: 'Avg Latency', value: '1,450ms', change: '+120ms', positive: false },
  ]

  const providerHealth = [
    { name: 'Bailian CN', status: 'healthy', latency: '340ms' },
    { name: 'Bailian INTL', status: 'healthy', latency: '520ms' },
    { name: 'GA Accelerator', status: 'healthy', latency: '180ms' },
  ]

  return (
    <div>
      <h1 className="text-2xl font-bold text-white mb-6">Admin Overview</h1>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.map(({ label, value, change, positive }) => (
          <div key={label} className="card p-5">
            <p className="text-sm text-gray-500">{label}</p>
            <p className="text-2xl font-bold text-white mt-1">{value}</p>
            <span className={cn('text-xs font-medium', positive ? 'text-green-400' : 'text-red-400')}>
              {change}
            </span>
          </div>
        ))}
      </div>

      <div className="grid lg:grid-cols-2 gap-6">
        {/* Provider Health */}
        <div className="card p-6">
          <h3 className="text-sm font-medium text-gray-400 mb-4">Provider Health</h3>
          <div className="space-y-3">
            {providerHealth.map(({ name, status, latency }) => (
              <div key={name} className="flex items-center justify-between py-2 border-b border-gray-800/50 last:border-0">
                <div className="flex items-center gap-2">
                  {status === 'healthy' ? (
                    <CheckCircle className="w-4 h-4 text-green-400" />
                  ) : status === 'degraded' ? (
                    <AlertTriangle className="w-4 h-4 text-yellow-400" />
                  ) : (
                    <XCircle className="w-4 h-4 text-red-400" />
                  )}
                  <span className="text-sm text-white">{name}</span>
                </div>
                <span className="text-xs font-mono text-gray-500">{latency}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Top Models */}
        <div className="card p-6">
          <h3 className="text-sm font-medium text-gray-400 mb-4">Top Models (24h)</h3>
          <div className="space-y-3">
            {[
              { model: 'qwen-max', requests: '12,400', cost: '$49.60' },
              { model: 'wan-2.7-image', requests: '8,200', cost: '$24.60' },
              { model: 'qwen3.7-flash', requests: '6,800', cost: '$0.68' },
              { model: 'wan2.7-i2v', requests: '3,100', cost: '$31.00' },
              { model: 'cosyvoice-v2', requests: '2,500', cost: '$2.50' },
            ].map(({ model, requests, cost }) => (
              <div key={model} className="flex items-center justify-between py-2 border-b border-gray-800/50 last:border-0">
                <code className="text-sm font-mono text-gray-300">{model}</code>
                <div className="flex items-center gap-4 text-xs">
                  <span className="text-gray-500">{requests} reqs</span>
                  <span className="text-brand-400 font-mono">{cost}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

// ===== Admin Models =====

function AdminModels() {
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-white">Model Management</h1>
        <button className="btn-primary">+ Add Model</button>
      </div>

      <div className="card overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-500">
              <th className="text-left px-5 py-3 font-medium">Model</th>
              <th className="text-left px-5 py-3 font-medium">Modality</th>
              <th className="text-left px-5 py-3 font-medium">Providers</th>
              <th className="text-right px-5 py-3 font-medium">Price</th>
              <th className="text-center px-5 py-3 font-medium">Status</th>
              <th className="text-center px-5 py-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {[
              { id: 'qwen-max', modality: 'text', providers: 'CN, INTL', price: '$0.004/1K', status: 'active' },
              { id: 'qwen3.7-max', modality: 'text', providers: 'CN, INTL', price: '$0.006/1K', status: 'active' },
              { id: 'wan-2.7-image', modality: 'image', providers: 'INTL', price: '$0.03/img', status: 'active' },
              { id: 'wan2.7-i2v', modality: 'video', providers: 'INTL', price: '$0.10/s', status: 'active' },
              { id: 'cosyvoice-v2', modality: 'audio', providers: 'CN', price: '$0.0001/char', status: 'active' },
            ].map(({ id, modality, providers, price, status }) => (
              <tr key={id} className="border-b border-gray-800/50 hover:bg-gray-800/20">
                <td className="px-5 py-3 font-mono text-gray-300">{id}</td>
                <td className="px-5 py-3 text-gray-400 capitalize">{modality}</td>
                <td className="px-5 py-3 text-gray-500">{providers}</td>
                <td className="px-5 py-3 text-right font-mono text-brand-400">{price}</td>
                <td className="px-5 py-3 text-center">
                  <span className="badge bg-green-500/20 text-green-400 border border-green-500/30">{status}</span>
                </td>
                <td className="px-5 py-3 text-center">
                  <button className="btn-ghost text-xs py-1">Edit</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
