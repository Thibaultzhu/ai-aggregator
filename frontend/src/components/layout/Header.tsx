import { Link, useLocation } from 'react-router-dom'
import { useState } from 'react'
import { Zap, Menu, X, ChevronDown } from 'lucide-react'

const navItems = [
  { label: 'Explore', href: '/models' },
  { label: 'Pricing', href: '/pricing' },
  { label: 'Playground', href: '/playground', badge: 'NEW' },
  {
    label: 'Resources',
    href: '#',
    children: [
      { label: 'API Docs', href: '/docs' },
      { label: 'SDK Examples', href: '/docs#sdks' },
      { label: 'Changelog', href: '/docs#changelog' },
    ],
  },
]

export function Header() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [resourcesOpen, setResourcesOpen] = useState(false)
  const location = useLocation()

  return (
    <header className="sticky top-0 z-50 bg-gray-950/80 backdrop-blur-xl border-b border-gray-800">
      {/* Promo banner */}
      <div className="bg-gradient-to-r from-brand-600/20 via-purple-600/20 to-pink-600/20 border-b border-gray-800">
        <div className="max-w-7xl mx-auto px-4 py-1.5 text-center text-sm text-gray-300">
          <span className="text-brand-400 font-medium">Qwen3.7 Max</span> now available with reasoning mode —{' '}
          <Link to="/models" className="text-white hover:underline">Try it now →</Link>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link to="/" className="flex items-center gap-2 group">
            <div className="w-8 h-8 bg-gradient-to-br from-brand-500 to-purple-600 rounded-lg flex items-center justify-center">
              <Zap className="w-5 h-5 text-white" />
            </div>
            <span className="text-lg font-bold text-white group-hover:text-brand-400 transition-colors">
              AI Aggregator
            </span>
          </Link>

          {/* Desktop nav */}
          <nav className="hidden md:flex items-center gap-1">
            {navItems.map((item) =>
              item.children ? (
                <div key={item.label} className="relative">
                  <button
                    onClick={() => setResourcesOpen(!resourcesOpen)}
                    className="btn-ghost flex items-center gap-1"
                  >
                    {item.label}
                    <ChevronDown className={`w-4 h-4 transition-transform ${resourcesOpen ? 'rotate-180' : ''}`} />
                  </button>
                  {resourcesOpen && (
                    <div className="absolute top-full left-0 mt-1 w-48 bg-gray-900 border border-gray-700 rounded-xl shadow-xl py-1 z-50">
                      {item.children.map((child) => (
                        <Link
                          key={child.href}
                          to={child.href}
                          onClick={() => setResourcesOpen(false)}
                          className="block px-4 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800"
                        >
                          {child.label}
                        </Link>
                      ))}
                    </div>
                  )}
                </div>
              ) : (
                <Link
                  key={item.href}
                  to={item.href}
                  className={`btn-ghost flex items-center gap-1.5 relative ${
                    location.pathname === item.href ? 'text-white' : ''
                  }`}
                >
                  {item.label}
                  {item.badge && (
                    <span className="bg-green-500/20 text-green-400 text-[10px] font-bold px-1.5 py-0.5 rounded-full border border-green-500/30">
                      {item.badge}
                    </span>
                  )}
                </Link>
              )
            )}
          </nav>

          {/* CTA */}
          <div className="hidden md:flex items-center gap-3">
            <Link to="/login" className="btn-ghost text-sm">Sign In</Link>
            <Link to="/login" className="btn-primary text-sm">Get Started Free</Link>
          </div>

          {/* Mobile toggle */}
          <button
            className="md:hidden p-2 text-gray-400 hover:text-white"
            onClick={() => setMobileOpen(!mobileOpen)}
          >
            {mobileOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="md:hidden border-t border-gray-800 bg-gray-950">
          <div className="px-4 py-4 space-y-1">
            {navItems.map((item) => (
              <Link
                key={item.href}
                to={item.children ? item.children[0].href : item.href}
                onClick={() => setMobileOpen(false)}
                className="block px-3 py-2 rounded-lg text-gray-300 hover:text-white hover:bg-gray-800"
              >
                {item.label}
              </Link>
            ))}
            <div className="pt-4 border-t border-gray-800 flex flex-col gap-2">
              <Link to="/login" className="btn-secondary text-center">Sign In</Link>
              <Link to="/login" className="btn-primary text-center">Get Started Free</Link>
            </div>
          </div>
        </div>
      )}
    </header>
  )
}
