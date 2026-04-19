'use client'

import { useState } from 'react'
import type { Bank } from '@/types'
import BankCard from '@/components/BankCard'
import CreateBankForm from '@/components/CreateBankForm'

interface BanksViewProps {
  banks: Bank[]
}

export default function BanksView({ banks: initialBanks }: BanksViewProps) {
  // Banks live in local state so mutations are reflected immediately.
  // initialBanks comes from the server on first render; after that, all
  // changes are applied locally via the callbacks below. revalidatePath in
  // the server actions keeps the server cache in sync for future page loads.
  const [banks, setBanks] = useState<Bank[]>(initialBanks)
  const [showCreateForm, setShowCreateForm] = useState(false)

  // Called by CreateBankForm once the server action returns the new bank.
  function handleCreate(bank: Bank) {
    setBanks((prev) => [bank, ...prev]) // newest first, matching the API's sort order
  }

  // Called by BankCard once the server action returns the updated bank.
  function handleUpdate(updated: Bank) {
    setBanks((prev) => prev.map((b) => (b.id === updated.id ? updated : b)))
  }

  // Called by BankCard once the server action confirms deletion.
  function handleDelete(bankId: string) {
    setBanks((prev) => prev.filter((b) => b.id !== bankId))
  }

  return (
    <div>
      {/* Page header */}
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Question Banks</h1>
          <p className="mt-1 text-sm text-gray-500">
            Reusable sets of questions you can attach to a game.
          </p>
        </div>

        <button
          onClick={() => setShowCreateForm(true)}
          disabled={showCreateForm}
          className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white shadow-sm hover:opacity-90 disabled:opacity-50"
        >
          + New Bank
        </button>
      </div>

      {/* Inline create form — shown when the button is clicked */}
      {showCreateForm && (
        <CreateBankForm
          onClose={() => setShowCreateForm(false)}
          onCreate={(bank) => {
            handleCreate(bank)
            setShowCreateForm(false)
          }}
        />
      )}

      {/* Bank grid */}
      {banks.length === 0 && !showCreateForm ? (
        <div className="rounded-xl border border-dashed border-gray-300 bg-white/60 py-20 text-center">
          <p className="text-sm font-medium text-gray-500">No question banks yet.</p>
          <p className="mt-1 text-sm text-gray-400">
            Create one to start building questions.
          </p>
          <button
            onClick={() => setShowCreateForm(true)}
            className="mt-5 rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white shadow-sm hover:opacity-90"
          >
            Create your first bank
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {banks.map((bank) => (
            <BankCard
              key={bank.id}
              bank={bank}
              onUpdate={handleUpdate}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}
    </div>
  )
}
