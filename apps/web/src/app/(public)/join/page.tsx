'use client'

// Public join page — no auth required.
// Players enter a game code and display name, then are redirected to /play/[code].

import { useState, useTransition } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Suspense } from 'react'

const API_BASE = (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, '')

function JoinForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const prefillCode = searchParams.get('code') ?? ''

  const [code, setCode] = useState(prefillCode.toUpperCase())
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isPending, startTransition] = useTransition()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    startTransition(async () => {
      try {
        const res = await fetch(`${API_BASE}/join`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: code.trim().toUpperCase(), display_name: name.trim() }),
        })

        if (!res.ok) {
          const body = await res.json().catch(() => ({ error: 'Unknown error' }))
          setError(body.error ?? 'Failed to join game')
          return
        }

        const data = await res.json()
        // Store session token so the play page can use it for WebSocket auth.
        sessionStorage.setItem(`quibble_session_${data.game_code}`, data.session_token)
        router.push(`/play/${data.game_code}`)
      } catch {
        setError('Could not reach the server — is the game host ready?')
      }
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-1">
        <label className="text-xs font-medium uppercase tracking-wide text-slate-500">
          Game Code
        </label>
        <input
          type="text"
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase().slice(0, 6))}
          placeholder="ABCD12"
          maxLength={6}
          required
          className="w-full rounded-lg border border-gray-200 px-4 py-3 text-center font-display text-2xl font-semibold uppercase tracking-[0.2em] text-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
        />
      </div>

      <div className="space-y-1">
        <label className="text-xs font-medium uppercase tracking-wide text-slate-500">
          Display Name
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value.slice(0, 32))}
          placeholder="Your name"
          maxLength={32}
          required
          className="w-full rounded-lg border border-gray-200 px-4 py-3 text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
        />
        <p className="text-right text-xs text-slate-400">{name.length}/32</p>
      </div>

      {error && (
        <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-brand-red">{error}</p>
      )}

      <button
        type="submit"
        disabled={isPending || code.length < 6 || name.trim().length === 0}
        className="w-full rounded-xl bg-brand-blue py-3 text-base font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
      >
        {isPending ? 'Joining…' : 'Join Game'}
      </button>
    </form>
  )
}

export default function JoinPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-100 px-4">
      <div className="w-full max-w-sm overflow-hidden rounded-2xl border border-gray-100 bg-white shadow-xl">
        <div className="h-[4px] bg-brand-blue" />
        <div className="px-8 py-8">
          <div className="mb-6 text-center">
            <p className="font-display text-3xl font-bold text-brand-blue">
              Qui<span className="italic">bb</span>le
            </p>
            <p className="mt-1 text-sm text-slate-500">Enter a code to join a game</p>
          </div>
          <Suspense>
            <JoinForm />
          </Suspense>
        </div>
      </div>
    </div>
  )
}
