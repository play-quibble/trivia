'use client'

import { useState, useTransition } from 'react'
import type { Question } from '@/types'
import {
  updateTextQuestionAction,
  updateMCQuestionAction,
  deleteQuestionAction,
} from '@/app/(host)/banks/[bankID]/actions'
import QuestionForm, { type QuestionFormValues } from '@/components/QuestionForm'

interface QuestionCardProps {
  bankId: string
  question: Question
  index: number
  total: number
  onUpdate: (q: Question) => void
  onDelete: (id: string) => void
  onMove: (id: string, direction: 'up' | 'down') => void
}

export default function QuestionCard({
  bankId,
  question,
  index,
  total,
  onUpdate,
  onDelete,
  onMove,
}: QuestionCardProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [isPending, startTransition] = useTransition()

  function handleDelete() {
    if (!confirm('Delete this question? This can\'t be undone.')) return
    startTransition(async () => {
      const result = await deleteQuestionAction(bankId, question.id)
      if (result.success) onDelete(question.id)
    })
  }

  if (isEditing) {
    return (
      <QuestionEditForm
        bankId={bankId}
        question={question}
        onSave={(updated) => {
          onUpdate(updated)
          setIsEditing(false)
        }}
        onCancel={() => setIsEditing(false)}
      />
    )
  }

  return (
    <div
      className={`overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm transition-opacity ${isPending ? 'opacity-50' : ''}`}
    >
      <div className="flex items-stretch">
        {/* Reorder controls — left strip */}
        <div className="flex flex-col items-center justify-center gap-0.5 border-r border-gray-100 px-2 py-3">
          <button
            onClick={() => onMove(question.id, 'up')}
            disabled={index === 0}
            className="rounded p-1 text-gray-300 hover:bg-slate-100 hover:text-gray-600 disabled:pointer-events-none disabled:opacity-0"
            aria-label="Move question up"
          >
            ▲
          </button>
          <span className="text-xs font-medium tabular-nums text-gray-400">
            {index + 1}
          </span>
          <button
            onClick={() => onMove(question.id, 'down')}
            disabled={index === total - 1}
            className="rounded p-1 text-gray-300 hover:bg-slate-100 hover:text-gray-600 disabled:pointer-events-none disabled:opacity-0"
            aria-label="Move question down"
          >
            ▼
          </button>
        </div>

        {/* Question body */}
        <div className="flex flex-1 flex-col p-4">
          {/* Header row: type badge + points + actions */}
          <div className="mb-3 flex items-center gap-2">
            <span
              className={`rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide ${
                question.type === 'multiple_choice'
                  ? 'bg-blue-50 text-blue-600'
                  : 'bg-emerald-50 text-emerald-600'
              }`}
            >
              {question.type === 'multiple_choice' ? 'MC' : 'Text'}
            </span>
            <span className="text-xs text-gray-400">{question.points} pts</span>

            <div className="ml-auto flex gap-1">
              <button
                onClick={() => setIsEditing(true)}
                className="rounded-lg px-2.5 py-1 text-xs font-medium text-gray-500 hover:bg-slate-100 hover:text-gray-700"
              >
                Edit
              </button>
              <button
                onClick={handleDelete}
                disabled={isPending}
                className="rounded-lg px-2.5 py-1 text-xs font-medium text-brand-red/80 hover:bg-red-50 hover:text-brand-red disabled:opacity-50"
              >
                Delete
              </button>
            </div>
          </div>

          {/* Prompt */}
          <p className="text-sm font-medium text-gray-900">{question.prompt}</p>

          {/* Answers / choices */}
          <div className="mt-3">
            {question.type === 'text' && question.accepted_answers && (
              <div className="flex flex-wrap gap-1.5">
                {question.accepted_answers.map((ans, i) => (
                  <span
                    key={i}
                    className={`rounded-md px-2 py-0.5 text-xs ${
                      i === 0
                        ? 'bg-emerald-50 font-medium text-emerald-700'   // primary answer
                        : 'bg-gray-50 text-gray-500'                     // alternates
                    }`}
                  >
                    {ans}
                  </span>
                ))}
              </div>
            )}

            {question.type === 'multiple_choice' && question.choices && (
              <div className="grid grid-cols-2 gap-1.5">
                {question.choices.map((choice, i) => (
                  <div
                    key={i}
                    className={`flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs ${
                      choice.correct
                        ? 'bg-emerald-50 font-medium text-emerald-700'
                        : 'bg-gray-50 text-gray-500'
                    }`}
                  >
                    <span className={`h-3 w-3 flex-shrink-0 rounded-full border ${choice.correct ? 'border-emerald-500 bg-emerald-500' : 'border-gray-300'}`} />
                    {choice.text}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// QuestionEditForm is the inline edit wrapper embedded inside QuestionCard.
// It supplies the edit card chrome and wires QuestionForm to the update actions.
// The question's type is fixed, so the form is rendered with `lockType`.
function QuestionEditForm({
  bankId,
  question,
  onSave,
  onCancel,
}: {
  bankId: string
  question: Question
  onSave: (q: Question) => void
  onCancel: () => void
}) {
  const [isPending, startTransition] = useTransition()

  function handleSubmit(values: QuestionFormValues) {
    startTransition(async () => {
      if (question.type === 'text') {
        const result = await updateTextQuestionAction(
          bankId, question.id, values.prompt, values.acceptedAnswers, values.points,
        )
        if (result.question) onSave(result.question)
      } else {
        const result = await updateMCQuestionAction(
          bankId, question.id, values.prompt, values.choices, values.points,
        )
        if (result.question) onSave(result.question)
      }
    })
  }

  return (
    <div className="overflow-hidden rounded-xl border border-brand-blue/30 bg-white shadow-md ring-1 ring-brand-blue/10">
      <div className="h-[3px] bg-brand-blue" />
      <QuestionForm
        initialValues={{
          type: question.type,
          prompt: question.prompt,
          points: question.points,
          acceptedAnswers: question.accepted_answers,
          choices: question.choices,
        }}
        lockType
        submitLabel="Save"
        isPending={isPending}
        className="space-y-4 p-5"
        onSubmit={handleSubmit}
        onCancel={onCancel}
      />
    </div>
  )
}
