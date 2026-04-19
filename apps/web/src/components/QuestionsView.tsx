'use client'

import { useState, useTransition } from 'react'
import type { Question } from '@/types'
import QuestionCard from '@/components/QuestionCard'
import AddQuestionForm from '@/components/AddQuestionForm'
import { reorderQuestionsAction } from '@/app/(host)/banks/[bankID]/actions'

interface QuestionsViewProps {
  bankId: string
  initialQuestions: Question[]
}

export default function QuestionsView({ bankId, initialQuestions }: QuestionsViewProps) {
  const [questions, setQuestions] = useState<Question[]>(initialQuestions)
  const [showAddForm, setShowAddForm] = useState(false)
  const [isReordering, startReorder] = useTransition()

  // Called by AddQuestionForm when a question is saved — prepend to list with
  // position already set by the API (appended to end), then sort to be safe.
  function handleCreate(q: Question) {
    setQuestions((prev) => [...prev, q].sort((a, b) => a.position - b.position))
    setShowAddForm(false)
  }

  // Called by QuestionCard when edits are saved.
  function handleUpdate(updated: Question) {
    setQuestions((prev) => prev.map((q) => (q.id === updated.id ? updated : q)))
  }

  // Called by QuestionCard when a question is deleted.
  function handleDelete(questionId: string) {
    setQuestions((prev) => prev.filter((q) => q.id !== questionId))
  }

  // Move a question up or down by swapping positions in local state and
  // persisting the new order to the API.
  function handleMove(questionId: string, direction: 'up' | 'down') {
    const idx = questions.findIndex((q) => q.id === questionId)
    if (idx === -1) return
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1
    if (swapIdx < 0 || swapIdx >= questions.length) return

    // Swap the two items to build the new order.
    const next = [...questions]
    ;[next[idx], next[swapIdx]] = [next[swapIdx], next[idx]]
    setQuestions(next)

    // Persist in the background. startReorder keeps isReordering true until
    // the server action resolves — we dim the list while it's in flight.
    const orderedIds = next.map((q) => q.id)
    startReorder(async () => {
      await reorderQuestionsAction(bankId, orderedIds)
    })
  }

  return (
    <div>
      {/* Add question button / form */}
      {showAddForm ? (
        <AddQuestionForm
          bankId={bankId}
          onCreate={handleCreate}
          onClose={() => setShowAddForm(false)}
        />
      ) : (
        <button
          onClick={() => setShowAddForm(true)}
          className="mb-6 rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white shadow-sm hover:opacity-90"
        >
          + Add Question
        </button>
      )}

      {/* Empty state */}
      {questions.length === 0 && !showAddForm && (
        <div className="rounded-xl border border-dashed border-gray-300 bg-white/60 py-20 text-center">
          <p className="text-sm font-medium text-gray-500">No questions yet.</p>
          <p className="mt-1 text-sm text-gray-400">
            Add your first question to this bank.
          </p>
        </div>
      )}

      {/* Question list */}
      {questions.length > 0 && (
        <div className={`space-y-3 transition-opacity ${isReordering ? 'opacity-70' : ''}`}>
          {questions.map((q, idx) => (
            <QuestionCard
              key={q.id}
              bankId={bankId}
              question={q}
              index={idx}
              total={questions.length}
              onUpdate={handleUpdate}
              onDelete={handleDelete}
              onMove={handleMove}
            />
          ))}
        </div>
      )}
    </div>
  )
}
