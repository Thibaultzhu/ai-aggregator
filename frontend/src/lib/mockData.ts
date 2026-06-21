import type { Model, PricingTier } from '@/types'

// ===== Mock Models Data =====
export const mockModels: Model[] = [
  // Text / LLM
  {
    id: '1', modelId: 'qwen-max', displayName: 'Qwen Max', provider: 'Alibaba',
    modality: 'text', capabilities: ['chat', 'completion', 'function_call'],
    pricing: { input: 0.004, output: 0.012, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 32768, maxOutput: 8192, supportsStream: true, isAsync: false,
    status: 'active', description: 'Alibaba\'s most capable language model for complex reasoning and generation',
    tags: ['reasoning', 'function-call'],
  },
  {
    id: '2', modelId: 'qwen3.7-max', displayName: 'Qwen3.7 Max', provider: 'Alibaba',
    modality: 'text', capabilities: ['chat', 'completion', 'reasoning'],
    pricing: { input: 0.006, output: 0.018, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 131072, maxOutput: 16384, supportsStream: true, isAsync: false,
    status: 'active', description: 'Latest Qwen with enhanced reasoning capabilities',
    tags: ['reasoning', 'thinking'], discount: 15,
  },
  {
    id: '3', modelId: 'qwen3.7-flash', displayName: 'Qwen3.7 Flash', provider: 'Alibaba',
    modality: 'text', capabilities: ['chat', 'completion', 'reasoning'],
    pricing: { input: 0.0001, output: 0.0003, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 131072, maxOutput: 16384, supportsStream: true, isAsync: false,
    status: 'active', description: 'Ultra-fast and affordable for everyday tasks',
    tags: ['fast', 'affordable'],
  },
  {
    id: '4', modelId: 'deepseek-r1', displayName: 'DeepSeek R1', provider: 'DeepSeek',
    modality: 'text', capabilities: ['chat', 'reasoning'],
    pricing: { input: 0.002, output: 0.008, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 65536, maxOutput: 8192, supportsStream: true, isAsync: false,
    status: 'active', description: 'Advanced reasoning model with chain-of-thought',
    tags: ['reasoning', 'thinking'],
  },
  {
    id: '5', modelId: 'glm-5.1', displayName: 'GLM-5.1', provider: 'Zhipu AI',
    modality: 'text', capabilities: ['chat', 'completion'],
    pricing: { input: 0.004, output: 0.004, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 128000, maxOutput: 16000, supportsStream: false, isAsync: false,
    status: 'active', description: 'Zhipu AI flagship model with strong Chinese understanding',
    tags: ['chinese', 'enterprise'],
  },
  // Image
  {
    id: '10', modelId: 'wan-2.7-image-pro', displayName: 'Wan 2.7 Image Pro', provider: 'Alibaba',
    modality: 'image', capabilities: ['text_to_image', 'image_to_image'],
    pricing: { perImage: 0.075, unit: 'per_image', currency: 'USD' },
    supportsStream: false, isAsync: true, status: 'active',
    description: 'Professional-grade image generation with high fidelity',
    thumbnail: 'https://picsum.photos/seed/wan-img/400/300', tags: ['high-quality'],
  },
  {
    id: '11', modelId: 'wan-image', displayName: 'Wan Image', provider: 'Alibaba',
    modality: 'image', capabilities: ['text_to_image'],
    pricing: { perImage: 0.03, unit: 'per_image', currency: 'USD' },
    supportsStream: false, isAsync: true, status: 'active',
    description: 'Fast and affordable text-to-image generation',
    thumbnail: 'https://picsum.photos/seed/wan-img2/400/300', tags: ['fast', 'affordable'],
  },
  {
    id: '12', modelId: 'wan-image-edit', displayName: 'Wan Image Edit', provider: 'Alibaba',
    modality: 'image', capabilities: ['image_edit'],
    pricing: { perImage: 0.03, unit: 'per_image', currency: 'USD' },
    supportsStream: false, isAsync: true, status: 'active',
    description: 'Edit and transform existing images with AI',
    thumbnail: 'https://picsum.photos/seed/wan-edit/400/300', tags: ['editing'],
  },
  // Video
  {
    id: '20', modelId: 'wan2.7-i2v', displayName: 'Wan 2.7 I2V', provider: 'Alibaba',
    modality: 'video', capabilities: ['image_to_video'],
    pricing: { perSecond: 0.10, unit: 'per_second', currency: 'USD' },
    supportsStream: false, isAsync: true, status: 'active',
    description: 'Convert images to stunning videos with AI motion',
    thumbnail: 'https://picsum.photos/seed/wan-i2v/400/300', tags: ['image-to-video'], discount: 10,
  },
  {
    id: '21', modelId: 'wan2.7-t2v', displayName: 'Wan 2.7 T2V', provider: 'Alibaba',
    modality: 'video', capabilities: ['text_to_video'],
    pricing: { perSecond: 0.086, unit: 'per_second', currency: 'USD' },
    supportsStream: false, isAsync: true, status: 'active',
    description: 'Generate videos from text descriptions',
    thumbnail: 'https://picsum.photos/seed/wan-t2v/400/300', tags: ['text-to-video'],
  },
  // Audio
  {
    id: '30', modelId: 'cosyvoice-v2', displayName: 'CosyVoice V2', provider: 'Alibaba',
    modality: 'audio', capabilities: ['text_to_speech'],
    pricing: { perCharacter: 0.0001, unit: 'per_character', currency: 'USD' },
    supportsStream: true, isAsync: false, status: 'active',
    description: 'Natural-sounding text-to-speech with multiple voices',
    tags: ['tts', 'natural'],
  },
  {
    id: '31', modelId: 'paraformer-v2', displayName: 'Paraformer V2', provider: 'Alibaba',
    modality: 'audio', capabilities: ['speech_to_text'],
    pricing: { perSecond: 0.006, unit: 'per_second', currency: 'USD' },
    supportsStream: true, isAsync: false, status: 'active',
    description: 'High-accuracy speech recognition for multiple languages',
    tags: ['asr', 'multilingual'],
  },
  // Embedding
  {
    id: '40', modelId: 'text-embedding-v3', displayName: 'Text Embedding V3', provider: 'Alibaba',
    modality: 'embedding', capabilities: ['embedding'],
    pricing: { input: 0.0001, unit: 'per_1k_tokens', currency: 'USD' },
    maxContext: 8192, supportsStream: false, isAsync: false, status: 'active',
    description: 'State-of-the-art text embeddings for search and RAG',
    tags: ['embedding', 'rag'],
  },
]

// ===== Category Definitions =====
export const categories = [
  { id: 'all', label: 'All Models', count: mockModels.length },
  { id: 'text', label: 'Text / LLM', count: mockModels.filter(m => m.modality === 'text').length },
  { id: 'image', label: 'Image Generation', count: mockModels.filter(m => m.modality === 'image').length },
  { id: 'video', label: 'Video Generation', count: mockModels.filter(m => m.modality === 'video').length },
  { id: 'audio', label: 'Audio / Speech', count: mockModels.filter(m => m.modality === 'audio').length },
  { id: 'embedding', label: 'Embeddings', count: mockModels.filter(m => m.modality === 'embedding').length },
]

// ===== Pricing Tiers =====
export const pricingTiers: PricingTier[] = [
  { name: 'Bronze', deposit: 0, concurrency: 5, rateLimit: '60 RPM' },
  { name: 'Silver', deposit: 100, concurrency: 20, rateLimit: '300 RPM' },
  { name: 'Gold', deposit: 1000, concurrency: 100, rateLimit: '1,000 RPM' },
  { name: 'Ultra', deposit: 10000, concurrency: 500, rateLimit: '10,000 RPM' },
]

// ===== Mock Usage Data =====
export const mockUsageSummary = {
  totalRequests: 12847,
  totalCost: 42.35,
  totalTokens: 2_450_000,
  byModel: {
    'qwen-max': { requests: 5200, cost: 18.50 },
    'wan-2.7-image': { requests: 3100, cost: 9.30 },
    'wan2.7-i2v': { requests: 850, cost: 8.50 },
    'cosyvoice-v2': { requests: 2400, cost: 3.20 },
    'text-embedding-v3': { requests: 1297, cost: 2.85 },
  },
  dailyTrend: Array.from({ length: 7 }, (_, i) => ({
    date: new Date(Date.now() - (6 - i) * 86400000).toISOString().split('T')[0],
    requests: Math.floor(1500 + Math.random() * 1000),
    cost: Math.floor(4 + Math.random() * 5),
  })),
}

// ===== Mock API Keys =====
export const mockApiKeys = [
  {
    id: '1', name: 'Production Key', keyPrefix: 'sk-aggr-a8f2',
    permissions: { models: '*' }, rateLimitRpm: 60, isActive: true,
    lastUsedAt: '2026-06-19T08:30:00Z', createdAt: '2026-05-01T00:00:00Z',
  },
  {
    id: '2', name: 'Dev / Testing', keyPrefix: 'sk-aggr-b3c1',
    permissions: { models: ['qwen-max', 'qwen-plus'] }, rateLimitRpm: 30,
    isActive: true, lastUsedAt: '2026-06-18T14:20:00Z', createdAt: '2026-06-01T00:00:00Z',
  },
]
