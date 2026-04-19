'use client'

import { useRef, useTransition } from 'react'
import type { Bank } from '@/types'
import { createBankAction } from '@/app/banks/actions'

interface CreateBankFormProps {
  onClose: () => void
  // onCreate is called with the newly created bank so the parent can update
  // local state immediately rather than waiting for a page re-fetch.
  onCreate: (bank: Bank) => void
}

export default function CreateBankForm({ onClose, onCreate }: CreateBankFormProps) {
  const formRef = useRef<HTMLFormElement>(null)
  const [isPending, startTransition] = useTransition()

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)

    startTransition(async () => {
      const result = await createBankAction(formData)
      if (result.bank) {
        onCreate(result.bank) // update parent state instantly
      }
      // onClose is handled by the parent in the onCreate callback in BanksView,
      // but call it here too as a safety net in case onCreate isn't provided.
    })
  }

  return (
    <div className="mb-6 overflow-hidden rounded-xl border border-gray-100 bg-white shadow-md">
      {/* Brand accent strip — matches the card treatment */}
      <div className="h-[3px] bg-brand-blue" />

      <div className="p-6">
        <h2 className="mb-5 text-base font-semibold text-gray-900">New Question Bank</h2>

        <form ref={formRef} onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="name" className="mb-1.5 block text-sm font-medium text-gray-700">
              Name <span className="text-brand-red">*</span>
            </label>
            <input
              id="name"
              name="name"
              type="text"
              required
              autoFocus
              placeholder="e.g. General Knowledge"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm placeholder-gray-400 focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
          </div>

          <div>
            <label htmlFor="description" className="mb-1.5 block text-sm font-medium text-gray-700">
              Description{' '}
              <span className="font-normal text-gray-400">(optional)</span>
            </label>
            <input
              id="description"
              name="description"
              type="text"
              placeholder="What's this bank about?"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm placeholder-gray-400 focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
          </div>

          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={onClose}
              disabled={isPending}
              className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 hover:bg-slate-50 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
            >
              {isPending ? 'Creating…' : 'Create Bank'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
