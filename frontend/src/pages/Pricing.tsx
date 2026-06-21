import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Zap } from 'lucide-react'
import { mockModels, pricingTiers } from '@/lib/mockData'
import { formatCurrency, cn } from '@/lib/utils'

export default function Pricing() {
  const [activeTab, setActiveTab] = useState<'text' | 'image' | 'video' | 'audio'>('text')

  const textModels = mockModels.filter(m => m.modality === 'text')
  const imageModels = mockModels.filter(m => m.modality === 'image')
  const videoModels = mockModels.filter(m => m.modality === 'video')
  const audioModels = mockModels.filter(m => m.modality === 'audio')

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
      {/* Header */}
      <div className="text-center mb-16">
        <h1 className="text-4xl font-bold text-white">Simple, Transparent Pricing</h1>
        <p className="mt-3 text-gray-400 max-w-xl mx-auto">
          Pay only for what you use. No monthly fees, no hidden costs.
          Start with $5 free credits.
        </p>
      </div>

      {/* Pricing Tiers */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-20">
        {pricingTiers.map((tier) => (
          <div
            key={tier.name}
            className={cn(
              'card p-6',
              tier.name === 'Gold' && 'border-brand-500/40 ring-1 ring-brand-500/20'
            )}
          >
            {tier.name === 'Gold' && (
              <span className="badge bg-brand-600 text-white mb-3">Popular</span>
            )}
            <h3 className="text-lg font-bold text-white">{tier.name}</h3>
            <div className="mt-2">
              <span className="text-3xl font-bold text-white">
                {tier.deposit === 0 ? 'Free' : `$${tier.deposit.toLocaleString()}`}
              </span>
              {tier.deposit > 0 && <span className="text-sm text-gray-500 ml-1">deposit</span>}
            </div>
            <ul className="mt-4 space-y-2 text-sm text-gray-400">
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> {tier.concurrency} concurrent</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> {tier.rateLimit}</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> All models</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> Pay per use</li>
            </ul>
            <Link
              to="/login"
              className={cn(
                'mt-6 block text-center py-2.5 rounded-lg text-sm font-medium transition-colors',
                tier.name === 'Gold' ? 'btn-primary' : 'btn-secondary'
              )}
            >
              {tier.deposit === 0 ? 'Start Free' : `Deposit $${tier.deposit}`}
            </Link>
          </div>
        ))}
      </div>

      {/* Model Pricing Tables */}
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-white mb-6">Per-Model Pricing</h2>

        {/* Tabs */}
        <div className="flex items-center gap-1 mb-6 bg-gray-900 rounded-xl p-1 w-fit">
          {([
            { id: 'text', label: 'Language Models' },
            { id: 'image', label: 'Image Generation' },
            { id: 'video', label: 'Video Generation' },
            { id: 'audio', label: 'Audio / Speech' },
          ] as const).map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                activeTab === tab.id
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-500 hover:text-gray-300'
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Text models table */}
        {activeTab === 'text' && (
          <div className="card overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-800 text-sm text-gray-500">
                  <th className="text-left px-6 py-3 font-medium">Model</th>
                  <th className="text-left px-6 py-3 font-medium">Context</th>
                  <th className="text-right px-6 py-3 font-medium">Input / 1K tokens</th>
                  <th className="text-right px-6 py-3 font-medium">Output / 1K tokens</th>
                  <th className="text-center px-6 py-3 font-medium">Stream</th>
                </tr>
              </thead>
              <tbody>
                {textModels.map((m) => (
                  <tr key={m.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-white">{m.displayName}</div>
                      <div className="text-xs text-gray-500">{m.provider} · {m.modelId}</div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">{(m.maxContext! / 1000).toFixed(0)}K</td>
                    <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatCurrency(m.pricing.input!)}</td>
                    <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatCurrency(m.pricing.output!)}</td>
                    <td className="px-6 py-4 text-center">{m.supportsStream ? '✅' : '❌'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="px-6 py-3 text-xs text-gray-600 bg-gray-900/50">1K tokens ≈ 750 words</p>
          </div>
        )}

        {/* Image models table */}
        {activeTab === 'image' && (
          <div className="card overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-800 text-sm text-gray-500">
                  <th className="text-left px-6 py-3 font-medium">Model</th>
                  <th className="text-left px-6 py-3 font-medium">Capabilities</th>
                  <th className="text-right px-6 py-3 font-medium">Per Image</th>
                  <th className="text-right px-6 py-3 font-medium">Per $1</th>
                </tr>
              </thead>
              <tbody>
                {imageModels.map((m) => (
                  <tr key={m.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-white">{m.displayName}</div>
                      <div className="text-xs text-gray-500">{m.modelId}</div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex gap-1 flex-wrap">
                        {m.capabilities.map(c => (
                          <span key={c} className="badge bg-gray-800 text-gray-400 text-xs">{c}</span>
                        ))}
                      </div>
                    </td>
                    <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatCurrency(m.pricing.perImage!)}</td>
                    <td className="px-6 py-4 text-sm text-right text-gray-400">{Math.floor(1 / m.pricing.perImage!)} imgs</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Video models table */}
        {activeTab === 'video' && (
          <div className="card overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-800 text-sm text-gray-500">
                  <th className="text-left px-6 py-3 font-medium">Model</th>
                  <th className="text-left px-6 py-3 font-medium">Type</th>
                  <th className="text-right px-6 py-3 font-medium">Per Second</th>
                  <th className="text-right px-6 py-3 font-medium">Per $1</th>
                </tr>
              </thead>
              <tbody>
                {videoModels.map((m) => (
                  <tr key={m.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-white">{m.displayName}</div>
                      <div className="text-xs text-gray-500">{m.modelId}</div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">{m.capabilities.join(', ')}</td>
                    <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatCurrency(m.pricing.perSecond!)}</td>
                    <td className="px-6 py-4 text-sm text-right text-gray-400">{Math.floor(1 / m.pricing.perSecond!)} sec</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Audio models table */}
        {activeTab === 'audio' && (
          <div className="card overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-800 text-sm text-gray-500">
                  <th className="text-left px-6 py-3 font-medium">Model</th>
                  <th className="text-left px-6 py-3 font-medium">Type</th>
                  <th className="text-right px-6 py-3 font-medium">Price</th>
                  <th className="text-center px-6 py-3 font-medium">Stream</th>
                </tr>
              </thead>
              <tbody>
                {audioModels.map((m) => (
                  <tr key={m.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-white">{m.displayName}</div>
                      <div className="text-xs text-gray-500">{m.modelId}</div>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">{m.capabilities.join(', ')}</td>
                    <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">
                      {m.pricing.perSecond
                        ? `${formatCurrency(m.pricing.perSecond)}/sec`
                        : `${formatCurrency(m.pricing.perCharacter!)}/char`
                      }
                    </td>
                    <td className="px-6 py-4 text-center">{m.supportsStream ? '✅' : '❌'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* FAQ */}
      <div className="mt-16 max-w-3xl mx-auto">
        <h2 className="text-2xl font-bold text-white mb-8 text-center">Frequently Asked Questions</h2>
        <div className="space-y-4">
          {[
            { q: 'How does billing work?', a: 'We use a pay-per-use model. Add credits to your account and start creating. You\'re only charged for actual usage.' },
            { q: 'Is there a free tier?', a: 'Yes! New users get $5 in free credits with no credit card required.' },
            { q: 'What API format do you use?', a: 'We expose OpenAI-compatible APIs, so you can use any OpenAI SDK or compatible library with just a URL change.' },
            { q: 'Do you support streaming?', a: 'Yes, all text/LLM models support SSE streaming. Audio models support real-time streaming as well.' },
          ].map(({ q, a }) => (
            <details key={q} className="card group">
              <summary className="px-6 py-4 cursor-pointer text-white font-medium hover:text-brand-400 transition-colors list-none flex items-center justify-between">
                {q}
                <span className="text-gray-600 group-open:rotate-180 transition-transform">▾</span>
              </summary>
              <p className="px-6 pb-4 text-sm text-gray-400 leading-relaxed">{a}</p>
            </details>
          ))}
        </div>
      </div>
    </div>
  )
}
