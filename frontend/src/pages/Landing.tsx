import { Link } from 'react-router-dom'
import { ArrowRight, Zap, Globe, Shield, Cpu, Code2, Image, Video, Mic, MessageSquare } from 'lucide-react'
import { mockModels } from '@/lib/mockData'
import { formatCurrency, getModalityColor } from '@/lib/utils'

export default function Landing() {
  return (
    <div>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-brand-600/5 via-transparent to-transparent" />
        <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-20 pb-24">
          <div className="text-center max-w-4xl mx-auto">
            <h1 className="text-5xl sm:text-6xl font-extrabold tracking-tight">
              <span className="text-white">Ultimate AI</span>
              <br />
              <span className="bg-gradient-to-r from-brand-400 via-purple-400 to-pink-400 bg-clip-text text-transparent">
                Model Aggregation Platform
              </span>
            </h1>
            <p className="mt-6 text-lg text-gray-400 max-w-2xl mx-auto leading-relaxed">
              Access 1000+ top-tier AI models through a single OpenAI-compatible API.
              Text, image, video, audio — one key, unlimited creativity.
            </p>
            <div className="mt-8 flex items-center justify-center gap-4">
              <Link to="/models" className="btn-primary flex items-center gap-2">
                Explore Models <ArrowRight className="w-4 h-4" />
              </Link>
              <Link to="/docs" className="btn-secondary flex items-center gap-2">
                <Code2 className="w-4 h-4" /> API Docs
              </Link>
            </div>

            {/* Trending tags */}
            <div className="mt-6 flex items-center justify-center gap-2 flex-wrap">
              <span className="text-sm text-gray-500">Trending:</span>
              <Link to="/models?category=image" className="badge-hot cursor-pointer">
                <Image className="w-3 h-3 mr-1" /> Image Gen
              </Link>
              <Link to="/models?category=video" className="badge-hot cursor-pointer">
                <Video className="w-3 h-3 mr-1" /> Video Gen
              </Link>
              <Link to="/models?category=text" className="badge-new cursor-pointer">
                <MessageSquare className="w-3 h-3 mr-1" /> LLM
              </Link>
            </div>
          </div>

          {/* Interactive demo mockup */}
          <div className="mt-16 max-w-4xl mx-auto">
            <DemoCard />
          </div>
        </div>
      </section>

      {/* Featured Models */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h2 className="text-3xl font-bold text-white">Featured Models</h2>
            <p className="mt-1 text-gray-500">Hand-picked models ready to use</p>
          </div>
          <Link to="/models" className="btn-ghost flex items-center gap-1">
            View all <ArrowRight className="w-4 h-4" />
          </Link>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {mockModels.slice(0, 8).map((model) => (
            <ModelCard key={model.id} model={model} />
          ))}
        </div>
      </section>

      {/* Provider Marquee */}
      <section className="border-y border-gray-800 bg-gray-900/30 py-12">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <p className="text-center text-sm text-gray-500 mb-8">Powered by leading AI providers</p>
          <div className="flex items-center justify-center gap-8 flex-wrap opacity-60">
            {['Alibaba Cloud', 'Zhipu AI', 'DeepSeek', 'Baidu', 'Moonshot', 'MiniMax', 'Baichuan'].map((name) => (
              <span key={name} className="text-lg font-semibold text-gray-400 hover:text-gray-200 transition-colors cursor-pointer">
                {name}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* Built For Developers */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold text-white">Built For Developers</h2>
          <p className="mt-2 text-gray-500">One API call. Every model. Zero infrastructure.</p>
        </div>

        <div className="grid lg:grid-cols-2 gap-8 items-center">
          {/* Code example */}
          <div className="card p-1">
            <div className="flex items-center gap-1 px-4 py-2 border-b border-gray-800">
              {['cURL', 'Python', 'Node.js'].map((lang, i) => (
                <button
                  key={lang}
                  className={`px-3 py-1 rounded text-xs font-medium ${
                    i === 0 ? 'bg-gray-800 text-white' : 'text-gray-500 hover:text-gray-300'
                  }`}
                >
                  {lang}
                </button>
              ))}
            </div>
            <pre className="p-4 text-sm font-mono text-gray-300 overflow-x-auto">
{`curl https://api.aggregator.com/v1/chat/completions \\
  -H "Authorization: Bearer sk-aggr-xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "qwen-max",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": true
  }'`}
            </pre>
          </div>

          {/* Response mockup */}
          <div className="card p-1">
            <div className="flex items-center gap-1 px-4 py-2 border-b border-gray-800">
              <span className="w-3 h-3 rounded-full bg-green-500" />
              <span className="text-xs text-gray-500 ml-2">200 OK — Streaming</span>
            </div>
            <pre className="p-4 text-sm font-mono text-gray-300 overflow-x-auto">
{`data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":"!"}}]}

data: {"choices":[{"delta":{"content":" How can"}}]}

data: {"choices":[{"delta":{"content":" I help?"}}]}

data: {"usage":{"total_tokens":25}}

data: [DONE]`}
            </pre>
          </div>
        </div>
      </section>

      {/* Core Features */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold text-white">Why AI Aggregator?</h2>
        </div>

        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {[
            { icon: Globe, title: '1000+ Models', desc: 'Text, image, video, audio, embedding — all modalities covered' },
            { icon: Zap, title: 'Fast Inference', desc: 'Optimized routing, connection pooling, and GA-accelerated endpoints' },
            { icon: Cpu, title: 'Auto Scaling', desc: 'Elastic infrastructure that scales with your demand, pay per use' },
            { icon: Shield, title: 'Enterprise Ready', desc: 'API key management, rate limiting, usage analytics, SOC 2' },
          ].map(({ icon: Icon, title, desc }) => (
            <div key={title} className="card-hover p-6">
              <div className="w-10 h-10 bg-brand-600/10 rounded-xl flex items-center justify-center mb-4">
                <Icon className="w-5 h-5 text-brand-400" />
              </div>
              <h3 className="font-semibold text-white mb-2">{title}</h3>
              <p className="text-sm text-gray-500 leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* CTA */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20">
        <div className="card bg-gradient-to-br from-brand-600/10 via-gray-900 to-purple-600/10 p-12 text-center">
          <h2 className="text-3xl font-bold text-white mb-4">Start Building Today</h2>
          <p className="text-gray-400 mb-8 max-w-lg mx-auto">
            Get $5 in free credits when you sign up. No credit card required.
          </p>
          <Link to="/login" className="btn-primary inline-flex items-center gap-2 text-lg px-8 py-3">
            Get Started Free <ArrowRight className="w-5 h-5" />
          </Link>
        </div>
      </section>
    </div>
  )
}

// ===== Sub-components =====

function DemoCard() {
  return (
    <div className="card border-gray-700 shadow-2xl shadow-brand-500/5">
      {/* Tabs */}
      <div className="flex items-center gap-1 px-4 py-3 border-b border-gray-800">
        {[
          { label: 'Image', icon: Image, active: true },
          { label: 'Video', icon: Video },
          { label: 'Audio', icon: Mic },
          { label: 'Text', icon: MessageSquare },
        ].map(({ label, icon: Icon, active }) => (
          <button
            key={label}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              active ? 'bg-brand-600 text-white' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
            }`}
          >
            <Icon className="w-3.5 h-3.5" /> {label}
          </button>
        ))}
      </div>
      {/* Content */}
      <div className="p-6 grid md:grid-cols-2 gap-6">
        <div>
          <label className="block text-sm font-medium text-gray-400 mb-2">Prompt</label>
          <textarea
            className="input h-32 resize-none"
            placeholder="A serene Japanese garden with cherry blossoms at sunset, watercolor style..."
            defaultValue="A serene Japanese garden with cherry blossoms at sunset, watercolor style"
          />
          <div className="mt-3 flex items-center gap-3">
            <select className="input w-auto text-sm">
              <option>wan-2.7-image-pro</option>
              <option>wan-image</option>
            </select>
            <select className="input w-auto text-sm">
              <option>1024x1024</option>
              <option>1536x1024</option>
            </select>
          </div>
          <button className="btn-primary w-full mt-4 flex items-center justify-center gap-2">
            <Zap className="w-4 h-4" /> Generate
          </button>
        </div>
        <div className="bg-gray-800/50 rounded-xl flex items-center justify-center min-h-[240px]">
          <div className="text-center text-gray-600">
            <Image className="w-12 h-12 mx-auto mb-2 opacity-30" />
            <p className="text-sm">Generated image will appear here</p>
          </div>
        </div>
      </div>
    </div>
  )
}

function ModelCard({ model }: { model: typeof mockModels[0] }) {
  const priceText = model.modality === 'text'
    ? `${formatCurrency(model.pricing.input!)}/1K in`
    : model.modality === 'image'
    ? `${formatCurrency(model.pricing.perImage!)}/img`
    : model.modality === 'video'
    ? `${formatCurrency(model.pricing.perSecond!)}/sec`
    : model.modality === 'audio'
    ? `${formatCurrency(model.pricing.perSecond || model.pricing.perCharacter!)}/${model.pricing.unit === 'per_second' ? 'sec' : 'char'}`
    : `${formatCurrency(model.pricing.input!)}/1K`

  return (
    <Link to={`/models#${model.modelId}`} className="card-hover group cursor-pointer">
      {model.thumbnail && (
        <div className="aspect-video bg-gray-800 overflow-hidden">
          <img
            src={model.thumbnail}
            alt={model.displayName}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
          />
        </div>
      )}
      <div className="p-4">
        <div className="flex items-center justify-between mb-2">
          <span className={`badge border ${getModalityColor(model.modality)}`}>
            {model.modality}
          </span>
          {model.discount && (
            <span className="badge-discount">{model.discount}% OFF</span>
          )}
        </div>
        <h3 className="font-semibold text-white group-hover:text-brand-400 transition-colors">
          {model.provider} / {model.modelId}
        </h3>
        <p className="text-sm text-gray-500 mt-1 line-clamp-2">{model.description}</p>
        <div className="mt-3 flex items-center justify-between">
          <span className="text-sm font-mono text-brand-400">{priceText}</span>
          <span className="text-xs text-gray-600">
            {model.supportsStream ? '⚡ Stream' : model.isAsync ? '⏳ Async' : ''}
          </span>
        </div>
      </div>
    </Link>
  )
}
