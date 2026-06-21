import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Wallet, ArrowUpRight, ArrowDownRight, Download } from 'lucide-react'
import * as api from '@/lib/api'
import type { BillingTransaction } from '@/lib/api'
import { formatCurrency } from '@/lib/utils'

/** Map tx_type to visual style */
function getTxStyle(txType: string) {
  switch (txType) {
    case 'credit_grant':
    case 'topup':
    case 'bonus':
      return {
        bg: 'bg-green-500/10',
        icon: <ArrowDownRight className="w-4 h-4 text-green-400" />,
        amountColor: 'text-green-400',
        badgeBg: 'bg-green-500/20 text-green-400 border-green-500/30',
        label: txType.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()),
      }
    case 'usage_charge':
    case 'charge':
      return {
        bg: 'bg-gray-800',
        icon: <ArrowUpRight className="w-4 h-4 text-gray-500" />,
        amountColor: 'text-gray-400',
        badgeBg: 'bg-red-500/20 text-red-400 border-red-500/30',
        label: 'Usage Charge',
      }
    case 'refund':
      return {
        bg: 'bg-blue-500/10',
        icon: <ArrowDownRight className="w-4 h-4 text-blue-400" />,
        amountColor: 'text-blue-400',
        badgeBg: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
        label: 'Refund',
      }
    default:
      return {
        bg: 'bg-gray-800',
        icon: <ArrowUpRight className="w-4 h-4 text-gray-500" />,
        amountColor: 'text-gray-400',
        badgeBg: 'bg-gray-700 text-gray-400 border-gray-600',
        label: txType.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()),
      }
  }
}

export default function Billing() {
  const navigate = useNavigate()
  const [balance, setBalance] = useState<number | null>(null)
  const [transactions, setTransactions] = useState<BillingTransaction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Auth guard: redirect to login if not authenticated
  useEffect(() => {
    if (!api.isAuthenticated()) {
      navigate('/login', { replace: true })
    }
  }, [navigate])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError(null)
      try {
        const [balRes, txRes] = await Promise.all([
          api.getBalance(),
          api.getBillingTransactions(),
        ])
        if (!cancelled) {
          setBalance(balRes.balance_usd)
          setTransactions(txRes.data)
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof api.ApiError && err.status === 401) {
            navigate('/login', { replace: true })
            return
          }
          if (err instanceof api.ApiError) {
            const body = err.body as { message?: string } | null
            setError(body?.message || err.statusText)
          } else {
            setError('Failed to load billing data.')
          }
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [navigate])

  const handleExportCsv = () => {
    if (transactions.length === 0) return
    const header = 'ID,Date,Type,Description,Amount (USD),Balance After (USD)\n'
    const rows = transactions
      .map(
        (tx) =>
          `${tx.id},${tx.created_at},${tx.tx_type},"${tx.description}",${tx.amount_usd},${tx.balance_after_usd}`,
      )
      .join('\n')
    const blob = new Blob([header + rows], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'billing-transactions.csv'
    a.click()
    URL.revokeObjectURL(url)
  }

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="w-10 h-10 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-500">Loading billing data...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Billing</h1>
        <p className="text-gray-500 mt-1">Manage your balance, payments, and invoices</p>
      </div>

      {/* Balance Card */}
      <div className="card p-6 mb-8 bg-gradient-to-br from-brand-600/10 to-gray-900">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-gray-400 flex items-center gap-2">
              <Wallet className="w-4 h-4" /> Current Balance
            </p>
            <p className="text-4xl font-bold text-white mt-2">
              {balance !== null ? formatCurrency(balance) : '--'}
            </p>
          </div>
          <div className="flex flex-col gap-2">
            <button className="btn-primary flex items-center gap-2">
              <ArrowUpRight className="w-4 h-4" /> Add Credits
            </button>
            <button
              onClick={handleExportCsv}
              className="btn-ghost text-sm flex items-center gap-2"
              disabled={transactions.length === 0}
            >
              <Download className="w-3 h-3" /> Export CSV
            </button>
          </div>
        </div>
      </div>

      {/* Transaction History */}
      <div className="card">
        <div className="px-6 py-4 border-b border-gray-800">
          <h3 className="text-sm font-medium text-gray-400">Transaction History</h3>
        </div>
        {transactions.length === 0 ? (
          <div className="p-12 text-center">
            <Wallet className="w-10 h-10 text-gray-700 mx-auto mb-3" />
            <p className="text-gray-500">No transactions yet</p>
            <p className="text-gray-600 text-sm mt-1">Your billing history will appear here.</p>
          </div>
        ) : (
          <div className="divide-y divide-gray-800/50">
            {transactions.map((tx) => {
              const style = getTxStyle(tx.tx_type)
              const isPositive = tx.amount_usd > 0
              return (
                <div
                  key={tx.id}
                  className="px-6 py-4 flex items-center gap-4 hover:bg-gray-800/20 transition-colors"
                >
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center ${style.bg}`}>
                    {style.icon}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-sm text-white truncate">
                        {tx.description || style.label}
                      </p>
                      <span className={`badge border text-xs ${style.badgeBg}`}>{style.label}</span>
                    </div>
                    <p className="text-xs text-gray-600 mt-0.5">
                      {new Date(tx.created_at).toLocaleString()}
                    </p>
                  </div>
                  <div className="text-right">
                    <span className={`text-sm font-mono ${style.amountColor}`}>
                      {isPositive ? '+' : ''}
                      {formatCurrency(tx.amount_usd)}
                    </span>
                    {tx.balance_after_usd !== undefined && tx.balance_after_usd !== null && (
                      <p className="text-xs text-gray-600 mt-0.5">
                        Bal: {formatCurrency(tx.balance_after_usd)}
                      </p>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
