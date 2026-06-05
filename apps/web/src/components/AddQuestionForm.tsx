'use client'

import { useTransition } from 'react'
import type { Question } from '@/types'
import { createTextQuestionAction, createMCQuestionAction } from '@/app/(host)/banks/[bankID]/actions'
import QuestionForm, { type QuestionFormValues } from '@/components/QuestionForm'

interface AddQuestionFormProps {
  bankId: string
  onCreate: (q: Question) => void
  onClose: () => void
}

// AddQuestionForm is the bank-page wrapper around QuestionForm: it supplies the
// card chrome and wires submission to the create server actions for `bankId`.
export default function AddQuestionForm({ bankId, onCreate, onClose }: AddQuestionFormProps) {
  const [isPending, startTransition] = useTransition()

  function handleSubmit(values: QuestionFormValues) {
    startTransition(async () => {
      if (values.type === 'text') {
        const result = await createTextQuestionAction(bankId, values.prompt, values.acceptedAnswers, values.points)
        if (result.question) onCreate(result.question)
      } else {
        const result = await createMCQuestionAction(bankId, values.prompt, values.choices, values.points)
        if (result.question) onCreate(result.question)
      }
    })
  }

  return (
    <div className="mb-6 overflow-hidden rounded-xl border border-gray-100 bg-white shadow-md">
      <div className="h-[3px] bg-brand-blue" />
      <div className="p-6">
        <QuestionForm
          title="New Question"
          submitLabel="Add Question"
          isPending={isPending}
          defaultPoints={1}
          autoFocusPrompt
          onSubmit={handleSubmit}
          onCancel={onClose}
        />
      </div>
    </div>
  )
}
