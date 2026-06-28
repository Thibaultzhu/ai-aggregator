import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Book, Terminal, Code2, ExternalLink } from 'lucide-react'
import * as api from '@/lib/api'

export default function Docs() {
  const [modelCount, setModelCount] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    async function loadCatalogSummary() {
      try {
        const res = await api.listMarketplaceModels()
        if (!cancelled) setModelCount(res.count)
      } catch {
        if (!cancelled) setModelCount(null)
      }
    }
    loadCatalogSummary()
    return () => { cancelled = true }
  }, [])

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
      <h1 className="text-4xl font-bold text-white mb-4">API Documentation</h1>
      <p className="text-gray-400 text-lg mb-12">
        Everything you need to integrate AI Aggregator into your application.
        {modelCount ? ` Current catalog: ${modelCount.toLocaleString()}+ active models.` : ''}
      </p>

      {/* Quick Start */}
      <section className="mb-16">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-2">
          <Terminal className="w-6 h-6 text-brand-400" /> Quick Start
        </h2>
        <div className="card p-6 space-y-4">
          <p className="text-gray-400">Get your API key and make your first request in under 2 minutes:</p>
          <div className="space-y-3">
            {[
              'Sign up and get your API key from the Dashboard',
              'Install your preferred HTTP client or SDK',
              'Make your first API call using the examples below',
            ].map((step, i) => (
              <div key={i} className="flex items-start gap-3">
                <span className="w-6 h-6 bg-brand-600/20 text-brand-400 rounded-full flex items-center justify-center text-xs font-bold shrink-0">
                  {i + 1}
                </span>
                <span className="text-sm text-gray-300">{step}</span>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Base URL */}
      <section className="mb-16">
        <h2 className="text-2xl font-bold text-white mb-6">Base URL</h2>
        <div className="code-block">
          <span className="text-gray-500"># Production</span>{'\n'}
          https://api.aggregator.com/v1{'\n\n'}
          <span className="text-gray-500"># Development</span>{'\n'}
          http://localhost:8081/v1
        </div>
      </section>

      {/* Authentication */}
      <section className="mb-16">
        <h2 className="text-2xl font-bold text-white mb-6">Authentication</h2>
        <p className="text-gray-400 mb-4">All requests require an API key in the Authorization header:</p>
        <div className="code-block">
          Authorization: Bearer sk-aggr-your-api-key-here
        </div>
      </section>

      {/* Endpoints */}
      <section className="mb-16" id="endpoints">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-2">
          <Code2 className="w-6 h-6 text-brand-400" /> Endpoints
        </h2>

        <div className="space-y-3">
          {[
            { method: 'POST', path: '/v1/chat/completions', desc: 'Text chat completion (streaming supported)' },
            { method: 'GET', path: '/v1/models', desc: 'List available models' },
            { method: 'POST', path: '/v1/images/generations', desc: 'Generate images (async)' },
            { method: 'GET', path: '/v1/images/generations/:id', desc: 'Get image generation result' },
            { method: 'POST', path: '/v1/video/generations', desc: 'Generate video (async)' },
            { method: 'GET', path: '/v1/video/generations/:id', desc: 'Get video generation result' },
            { method: 'POST', path: '/v1/audio/transcriptions', desc: 'Speech to text (ASR)' },
            { method: 'POST', path: '/v1/audio/speech', desc: 'Text to speech (TTS)' },
            { method: 'POST', path: '/v1/embeddings', desc: 'Generate text embeddings' },
            { method: 'POST', path: '/v1/files', desc: 'Upload file' },
            { method: 'GET', path: '/v1/files', desc: 'List uploaded files' },
            { method: 'GET', path: '/v1/files/:id', desc: 'Get file metadata' },
            { method: 'GET', path: '/v1/files/:id/content', desc: 'Download file content' },
            { method: 'DELETE', path: '/v1/files/:id', desc: 'Delete file' },
          ].map(({ method, path, desc }) => (
            <div key={path} className="card px-5 py-3 flex items-center gap-4 hover:border-gray-600 transition-colors">
              <span className={`text-xs font-bold px-2 py-0.5 rounded ${
                method === 'GET' ? 'bg-green-500/20 text-green-400' : 'bg-blue-500/20 text-blue-400'
              }`}>
                {method}
              </span>
              <code className="text-sm font-mono text-gray-300">{path}</code>
              <span className="text-sm text-gray-500 ml-auto hidden sm:block">{desc}</span>
            </div>
          ))}
        </div>
      </section>

      {/* SDK Examples */}
      <section className="mb-16" id="sdks">
        <h2 className="text-2xl font-bold text-white mb-6">SDK Examples</h2>
        <div className="grid sm:grid-cols-3 gap-4">
          {[
            { lang: 'Python', code: `from openai import OpenAI\n\nclient = OpenAI(\n    base_url="http://localhost:8081/v1",\n    api_key="sk-aggr-xxxx"\n)\n\nresponse = client.chat.completions.create(\n    model="qwen-max",\n    messages=[{"role": "user", "content": "Hello!"}]\n)` },
            { lang: 'Node.js', code: `import OpenAI from 'openai'\n\nconst client = new OpenAI({\n  baseURL: "http://localhost:8081/v1",\n  apiKey: "sk-aggr-xxxx"\n})\n\nconst response = await client.chat.completions.create({\n  model: "qwen-max",\n  messages: [{ role: "user", content: "Hello!" }]\n})` },
            { lang: 'cURL', code: `curl http://localhost:8081/v1/chat/completions \\\\\n  -H "Authorization: Bearer sk-aggr-xxxx" \\\\\n  -H "Content-Type: application/json" \\\\\n  -d '{"model":"qwen-max","messages":[{"role":"user","content":"Hello!"}]}'` },
          ].map(({ lang, code }) => (
            <div key={lang} className="card p-1">
              <div className="px-4 py-2 border-b border-gray-800 text-sm font-medium text-gray-300">{lang}</div>
              <pre className="p-4 text-xs font-mono text-gray-400 overflow-x-auto whitespace-pre-wrap">{code}</pre>
            </div>
          ))}
        </div>
      </section>

      {/* Error Codes */}
      <section className="mb-16">
        <h2 className="text-2xl font-bold text-white mb-6">Error Codes</h2>
        <div className="card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-500">
                <th className="text-left px-5 py-3 font-medium">Status</th>
                <th className="text-left px-5 py-3 font-medium">Code</th>
                <th className="text-left px-5 py-3 font-medium">Description</th>
              </tr>
            </thead>
            <tbody>
              {[
                ['400', 'invalid_request_error', 'Invalid request parameters'],
                ['401', 'authentication_error', 'Invalid or expired API key'],
                ['402', 'insufficient_balance', 'Account balance is too low'],
                ['429', 'rate_limit_exceeded', 'Too many requests'],
                ['500', 'internal_error', 'Server error'],
                ['502', 'upstream_error', 'Upstream provider error'],
              ].map(([status, code, desc]) => (
                <tr key={code} className="border-b border-gray-800/50">
                  <td className="px-5 py-3 font-mono text-red-400">{status}</td>
                  <td className="px-5 py-3 font-mono text-yellow-400">{code}</td>
                  <td className="px-5 py-3 text-gray-400">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* CTA */}
      <div className="card bg-gradient-to-r from-brand-600/10 to-purple-600/10 p-8 text-center">
        <h3 className="text-xl font-bold text-white mb-2">Ready to build?</h3>
        <p className="text-gray-400 mb-4">Get your API key and start creating in minutes.</p>
        <Link to="/dashboard/keys" className="btn-primary inline-flex items-center gap-2">
          Get API Key <ExternalLink className="w-4 h-4" />
        </Link>
      </div>
    </div>
  )
}
