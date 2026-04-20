'use client'

import Link from 'next/link'
import { useState, useTransition } from 'react'
import type { Bank } from '@/types'
import { updateBankAction, deleteBankAction } from '@/app/(host)/banks/actions'

interface BankCardProps {
  bank: Bank
  // Callbacks let the parent (BanksView) update its local bank list the moment
  // the server action returns, without waiting for a page re-fetch.
  onUpdate: (bank: Bank) => void
  onDelete: (bankId: string) => void
}

function formatDate(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  const diffDays = Math.floor(diffMs / 86_400_000)
  if (diffDays === 0) return 'today'
  if (diffDays === 1) return 'yesterday'
  if (diffDays < 30) return `${diffDays}d ago`
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export default function BankCard({ bank, onUpdate, onDelete }: BankCardProps) {
  const [isRenaming, setIsRenaming] = useState(false)
  const [isPending, startTransition] = useTransition()
  const [draftName, setDraftName] = useState(bank.name)
  const [draftDescription, setDraftDescription] = useState(bank.description ?? '')

  function handleRenameSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)

    startTransition(async () => {
      const result = await updateBankAction(bank.id, formData)
      if (result.bank) {
        onUpdate(result.bank) // notify parent to replace this card's data
        setIsRenaming(false)
      }
    })
  }

  function handleDelete() {
    if (!confirm(`Delete "${bank.name}"? This can't be undone.`)) return
    startTransition(async () => {
      const result = await deleteBankAction(bank.id)
      if (result.success) {
        onDelete(bank.id) // notify parent to remove this card from the list
      }
    })
  }

  // --- Rename (edit) mode ---
  if (isRenaming) {
    return (
      <div className="overflow-hidden rounded-xl border border-brand-blue/30 bg-white shadow-md ring-1 ring-brand-blue/10">
        {/* Brand accent strip */}
        <div className="h-[3px] bg-brand-blue" />
        <form onSubmit={handleRenameSubmit} className="space-y-3 p-5">
          <div>
            <label htmlFor={`name-${bank.id}`} className="mb-1 block text-xs font-medium text-gray-600">
              Name <span className="text-brand-red">*</span>
            </label>
            <input
              id={`name-${bank.id}`}
              name="name"
              type="text"
              required
              autoFocus
              value={draftName}
              onChange={(e) => setDraftName(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
          </div>
          <div>
            <label htmlFor={`desc-${bank.id}`} className="mb-1 block text-xs font-medium text-gray-600">
              Description
            </label>
            <input
              id={`desc-${bank.id}`}
              name="description"
              type="text"
              value={draftDescription}
              onChange={(e) => setDraftDescription(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => {
                setDraftName(bank.name)
                setDraftDescription(bank.description ?? '')
                setIsRenaming(false)
              }}
              disabled={isPending}
              className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:bg-gray-50 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending || !draftName.trim()}
              className="rounded-lg bg-brand-blue px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              {isPending ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    )
  }

  // --- View mode ---
  return (
    <div
      className={`group relative flex flex-col overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm transition-shadow duration-200 hover:shadow-md ${isPending ? 'opacity-50' : ''}`}
    >
      {/* Brand accent strip along the top edge */}
      <div className="h-[3px] bg-brand-blue" />

      <div className="flex flex-1 flex-col p-5">
        <div className="mb-4 flex-1">
          <Link href={`/banks/${bank.id}`} className="group/name">
            <h3 className="text-base font-semibold text-gray-900 group-hover/name:text-brand-blue">
              {bank.name}
            </h3>
          </Link>
          {bank.description ? (
            <p className="mt-1 text-sm text-gray-500">{bank.description}</p>
          ) : (
            <p className="mt-1 text-sm italic text-gray-400">No description</p>
          )}
        </div>

        <div className="flex items-center justify-between border-t border-gray-100 pt-3">
          <span className="text-xs text-gray-400">Created {formatDate(bank.created_at)}</span>

          <div className="flex gap-1">
            <Link
              href={`/banks/${bank.id}`}
              className="whitespace-nowrap rounded-lg px-2.5 py-1 text-xs font-medium text-brand-blue/80 hover:bg-blue-50 hover:text-brand-blue"
            >
              Open →
            </Link>
            <button
              onClick={() => setIsRenaming(true)}
              disabled={isPending}
              className="rounded-lg px-2.5 py-1 text-xs font-medium text-gray-500 hover:bg-slate-100 hover:text-gray-700 disabled:opacity-50"
              aria-label={`Edit ${bank.name}`}
            >
              Edit
            </button>
            <button
              onClick={handleDelete}
              disabled={isPending}
              className="rounded-lg px-2.5 py-1 text-xs font-medium text-brand-red/80 hover:bg-red-50 hover:text-brand-red disabled:opacity-50"
              aria-label={`Delete ${bank.name}`}
            >
              Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
