import { Routes, Route } from 'react-router-dom'
import { MainLayout } from '@/components/layout/MainLayout'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import Landing from '@/pages/Landing'
import Models from '@/pages/Models'
import Pricing from '@/pages/Pricing'
import Playground from '@/pages/Playground'
import Docs from '@/pages/Docs'
import Login from '@/pages/Login'
import Dashboard from '@/pages/Dashboard'
import ApiKeys from '@/pages/ApiKeys'
import Billing from '@/pages/Billing'
import Admin from '@/pages/Admin'

export default function App() {
  return (
    <Routes>
      {/* Public pages with main layout (header + footer) */}
      <Route element={<MainLayout />}>
        <Route path="/" element={<Landing />} />
        <Route path="/models" element={<Models />} />
        <Route path="/pricing" element={<Pricing />} />
        <Route path="/playground" element={<Playground />} />
        <Route path="/docs" element={<Docs />} />
        <Route path="/login" element={<Login />} />
      </Route>

      {/* Dashboard pages with sidebar layout */}
      <Route element={<DashboardLayout />}>
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/dashboard/keys" element={<ApiKeys />} />
        <Route path="/dashboard/billing" element={<Billing />} />
      </Route>

      {/* Admin (separate layout) */}
      <Route path="/admin/*" element={<Admin />} />
    </Routes>
  )
}
