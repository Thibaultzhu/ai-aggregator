import { clsx, type ClassValue } from 'clsx'

export function cn(...inputs: ClassValue[]) {
  return clsx(inputs)
}

export function formatCurrency(amount: number, currency = 'USD'): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(amount)
}

export function formatNumber(num: number): string {
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(1)}M`
  if (num >= 1_000) return `${(num / 1_000).toFixed(1)}K`
  return num.toString()
}

export function formatDate(date: string): string {
  return new Date(date).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function getModalityColor(modality: string): string {
  const colors: Record<string, string> = {
    text: 'text-blue-400 bg-blue-400/10 border-blue-400/30',
    image: 'text-purple-400 bg-purple-400/10 border-purple-400/30',
    video: 'text-pink-400 bg-pink-400/10 border-pink-400/30',
    audio: 'text-green-400 bg-green-400/10 border-green-400/30',
    embedding: 'text-yellow-400 bg-yellow-400/10 border-yellow-400/30',
  }
  return colors[modality] || 'text-gray-400 bg-gray-400/10 border-gray-400/30'
}

export function getModalityIcon(modality: string): string {
  const icons: Record<string, string> = {
    text: 'MessageSquare',
    image: 'Image',
    video: 'Video',
    audio: 'Mic',
    embedding: 'Binary',
  }
  return icons[modality] || 'Box'
}
