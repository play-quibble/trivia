'use client'

import { useState, useTransition } from 'react'
import Link from 'next/link'
import type { Game, Bank } from '@/types'
import { createGameAction } from '@/app/(host)/games/actions'

interface Props {
  games: Game[]
  banks: Bank[]
}

const STATUS_LABELS: Record<string, string> = {
  lobby:       'Lobby',
  in_progress: 'Live',
  completed:   'Completed',
  cancelled:   'Cancelled',
}

const STATUS_COLORS: Record<string, string> = {
  lobby:       'bg-amber-100 text-amber-700',
  in_progress: 'bg-green-100 text-green-700',
  completed:   'bg-slate-100 text-slate-500',
  cancelled:   'bg-red-100 text-red-500',
}

export default function GamesView({ games: initial, banks }: Props) {
  const [error, setError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [isPending, startTransition] = useTransition()

  function handleCreate(formData: FormData) {
    setError(null)
    startTransition(async () => {
      const result = await createGameAction(formData)
      if (result?.error) setError(result.error)
      // On success, createGameAction redirects — no local state update needed.
    })
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-10">

      {/* Page header */}
      <div className="mb-8 flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800">Games</h1>
        {banks.length > 0 && (
          <button
            onClick={() => setShowForm((v) => !v)}
            className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-brand-blue/90 transition-colors"
          >
            {showForm ? 'Cancel' : '+ New Game'}
          </button>
        )}
      </div>

      {/* No banks warning */}
      {banks.length === 0 && (
        <div className="mb-6 overflow-hidden rounded-xl border border-amber-100 bg-amber-50 px-6 py-4 text-sm text-amber-700">
          You need at least one question bank with questions before you can start a game.{' '}
          <Link href="/banks" className="font-medium underline">
            Create a bank →
          </Link>
        </div>
      )}

      {/* New game form */}
      {showForm && (
        <form
          action={handleCreate}
          className="mb-8 overflow-hidden rounded-xl border border-gray-100 bg-white shadow-md"
        >
          <div className="h-[3px] bg-brand-blue" />
          <div className="space-y-4 p-6">
            <h2 className="text-base font-semibold text-slate-800">New Game</h2>

            <div className="space-y-1">
              <label className="text-xs font-medium text-slate-500 uppercase tracking-wide">
                Question Bank
              </label>
              <select
                name="bank_id"
                required
                className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
              >
                {banks.map((b) => (
                  <option key={b.id} value={b.id}>{b.name}</option>
                ))}
              </select>
            </div>

            <div className="space-y-1">
              <label className="text-xs font-medium text-slate-500 uppercase tracking-wide">
                Questions Per Round
              </label>
              <input
                type="number"
                name="round_size"
                min={1}
                max={50}
                defaultValue={5}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
              />
              <p className="text-xs text-slate-400">
                A leaderboard is shown after every N questions.
              </p>
            </div>

            {error && (
              <p className="rounded-lg bg-red-50 px-4 py-2 text-sm text-brand-red">{error}</p>
            )}

            <div className="flex justify-end">
              <button
                type="submit"
                disabled={isPending}
                className="rounded-lg bg-brand-blue px-5 py-2 text-sm font-medium text-white hover:bg-brand-blue/90 disabled:opacity-50 transition-colors"
              >
                {isPending ? 'Creating…' : 'Create & Open Host Panel'}
              </button>
            </div>
          </div>
        </form>
      )}

      {/* Games list */}
      {initial.length === 0 && !showForm ? (
        <div className="overflow-hidden rounded-xl border border-dashed border-gray-200 bg-white/60 px-8 py-12 text-center text-slate-400">
          No games yet. Create one to get started.
        </div>
      ) : (
        <div className="space-y-3">
          {initial.map((g) => (
            <div
              key={g.id}
              className="flex items-center justify-between overflow-hidden rounded-xl border border-gray-100 bg-white px-5 py-4 shadow-sm"
            >
              <div className="flex items-center gap-4">
                {/* Game code */}
                <span className="font-display text-lg font-semibold tracking-widest text-brand-blue">
                  {g.code}
                </span>
                {/* Status badge */}
                <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS_COLORS[g.status] ?? ''}`}>
                  {STATUS_LABELS[g.status] ?? g.status}
                </span>
              </div>

              <div className="flex items-center gap-3">
                <span className="hidden text-xs text-slate-400 sm:block">
                  {new Date(g.created_at).toLocaleDateString()}
                </span>
                {(g.status === 'lobby' || g.status === 'in_progress') && (
                  <Link
                    href={`/games/${g.id}/host`}
                    className="rounded-lg bg-brand-blue px-3 py-1.5 text-xs font-medium text-white hover:bg-brand-blue/90 transition-colors"
                  >
                    Open Host Panel
                  </Link>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </main>
  )
}
