'use client'

import { useActionState, useEffect, useRef } from 'react'
import { updateProfile } from '@/app/(host)/account/actions'
import type { UserProfile } from '@/types'

interface Props {
  profile: UserProfile
}

export default function AccountForm({ profile }: Props) {
  const [state, action, pending] = useActionState(updateProfile, null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Focus the input when the form first mounts
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const saved = state?.ok === true
  const error = state?.ok === false ? state.error : null

  // Use the server-confirmed value after a successful save, otherwise the prop
  const currentDisplayName =
    (saved ? state.profile.display_name : profile.display_name) ?? ''

  return (
    <div className="space-y-6">

      {/* Profile info (read-only) */}
      <div className="rounded-2xl border border-gray-100 bg-white shadow-sm overflow-hidden">
        <div className="h-[3px] bg-brand-blue" />
        <div className="divide-y divide-gray-50 px-6">
          <Row label="Email" value={profile.email ?? '—'} />
          <Row
            label="Member since"
            value={new Date(profile.created_at).toLocaleDateString('en-US', {
              month: 'long', day: 'numeric', year: 'numeric',
            })}
          />
          <Row label="Account ID" value={profile.id} mono />
        </div>
      </div>

      {/* Edit display name */}
      <div className="rounded-2xl border border-gray-100 bg-white shadow-sm overflow-hidden">
        <div className="h-[3px] bg-brand-blue" />
        <div className="p-6">
          <h2 className="mb-1 text-sm font-semibold text-slate-800">Display name</h2>
          <p className="mb-4 text-xs text-slate-500">
            Shown in the navbar and used as your host identity. 1–50 characters.
          </p>

          <form action={action} className="space-y-3">
            <input
              ref={inputRef}
              name="display_name"
              type="text"
              defaultValue={currentDisplayName}
              maxLength={50}
              placeholder="e.g. Quiz Master Ben"
              className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-slate-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />

            {error && (
              <p className="text-xs text-brand-red">{error}</p>
            )}

            {saved && (
              <p className="text-xs text-emerald-600">✓ Display name saved.</p>
            )}

            <button
              type="submit"
              disabled={pending}
              className="rounded-xl bg-brand-blue px-5 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-50 transition-colors"
            >
              {pending ? 'Saving…' : 'Save'}
            </button>
          </form>
        </div>
      </div>

    </div>
  )
}

function Row({
  label,
  value,
  mono = false,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between py-3">
      <span className="text-sm text-slate-500">{label}</span>
      <span className={`text-sm text-slate-800 ${mono ? 'font-mono text-xs' : 'font-medium'}`}>
        {value}
      </span>
    </div>
  )
}
