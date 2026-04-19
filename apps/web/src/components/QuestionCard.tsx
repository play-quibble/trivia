'use client'

import { useState, useTransition } from 'react'
import type { Question, MCChoice } from '@/types'
import { MAX_PROMPT_LEN, MAX_CHOICE_LEN, MAX_ANSWER_LEN, MAX_CHOICES, MIN_CHOICES, MAX_ANSWERS } from '@/types'
import {
  updateTextQuestionAction,
  updateMCQuestionAction,
  deleteQuestionAction,
} from '@/app/(host)/banks/[bankID]/actions'

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

// QuestionEditForm is an inline edit form embedded inside QuestionCard.
// It mirrors the structure of AddQuestionForm but operates on an existing question.
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
  const [prompt, setPrompt] = useState(question.prompt)
  const [points, setPoints] = useState(String(question.points))

  // Text question state
  const [answers, setAnswers] = useState<string[]>(
    question.accepted_answers?.length ? question.accepted_answers : [''],
  )

  // MC question state
  const [choices, setChoices] = useState<MCChoice[]>(
    question.choices?.length
      ? question.choices
      : [
          { text: '', correct: true },
          { text: '', correct: false },
        ],
  )

  function addAnswer() {
    if (answers.length < MAX_ANSWERS) setAnswers([...answers, ''])
  }
  function removeAnswer(i: number) {
    if (answers.length <= 1) return
    setAnswers(answers.filter((_, idx) => idx !== i))
  }
  function setAnswer(i: number, val: string) {
    setAnswers(answers.map((a, idx) => (idx === i ? val : a)))
  }

  function addChoice() {
    if (choices.length < MAX_CHOICES) setChoices([...choices, { text: '', correct: false }])
  }
  function removeChoice(i: number) {
    if (choices.length <= MIN_CHOICES) return
    setChoices(choices.filter((_, idx) => idx !== i))
  }
  function setChoiceText(i: number, val: string) {
    setChoices(choices.map((c, idx) => (idx === i ? { ...c, text: val } : c)))
  }
  function setCorrectChoice(i: number) {
    setChoices(choices.map((c, idx) => ({ ...c, correct: idx === i })))
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const pts = Math.max(1, parseInt(points, 10) || 1000)

    startTransition(async () => {
      if (question.type === 'text') {
        const trimmed = answers.map((a) => a.trim()).filter(Boolean)
        const result = await updateTextQuestionAction(bankId, question.id, prompt.trim(), trimmed, pts)
        if (result.question) onSave(result.question)
      } else {
        const result = await updateMCQuestionAction(bankId, question.id, prompt.trim(), choices, pts)
        if (result.question) onSave(result.question)
      }
    })
  }

  return (
    <div className="overflow-hidden rounded-xl border border-brand-blue/30 bg-white shadow-md ring-1 ring-brand-blue/10">
      <div className="h-[3px] bg-brand-blue" />
      <form onSubmit={handleSubmit} className="space-y-4 p-5">
        {/* Prompt */}
        <div>
          <label className="mb-1.5 block text-sm font-medium text-gray-700">
            Question <span className="text-brand-red">*</span>
            <span className="ml-2 font-normal text-gray-400">
              {prompt.length}/{MAX_PROMPT_LEN}
            </span>
          </label>
          <textarea
            required
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            maxLength={MAX_PROMPT_LEN}
            rows={2}
            className="w-full resize-none rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
          />
        </div>

        {/* Points */}
        <div className="flex items-center gap-3">
          <label className="text-sm font-medium text-gray-700">Points</label>
          <input
            type="number"
            min={1}
            max={10000}
            value={points}
            onChange={(e) => setPoints(e.target.value)}
            className="w-24 rounded-lg border border-gray-200 px-3 py-1.5 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
          />
        </div>

        {/* Type-specific fields */}
        {question.type === 'text' ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-700">
              Accepted Answers <span className="text-brand-red">*</span>
              <span className="ml-2 font-normal text-gray-400 text-xs">(primary first, alternates/synonyms below)</span>
            </p>
            <div className="space-y-2">
              {answers.map((ans, i) => (
                <div key={i} className="flex gap-2">
                  <input
                    type="text"
                    value={ans}
                    onChange={(e) => setAnswer(i, e.target.value)}
                    maxLength={MAX_ANSWER_LEN}
                    placeholder={i === 0 ? 'Primary answer' : 'Alternate spelling or synonym'}
                    required={i === 0}
                    className="flex-1 rounded-lg border border-gray-200 px-3 py-1.5 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
                  />
                  {i > 0 && (
                    <button type="button" onClick={() => removeAnswer(i)}
                      className="rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-400 hover:bg-red-50 hover:text-brand-red">
                      ✕
                    </button>
                  )}
                </div>
              ))}
            </div>
            {answers.length < MAX_ANSWERS && (
              <button type="button" onClick={addAnswer}
                className="mt-2 text-xs font-medium text-brand-blue hover:underline">
                + Add alternate answer
              </button>
            )}
          </div>
        ) : (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-700">
              Choices <span className="text-brand-red">*</span>
              <span className="ml-2 font-normal text-gray-400 text-xs">(select the correct one)</span>
            </p>
            <div className="space-y-2">
              {choices.map((choice, i) => (
                <div key={i} className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="correct"
                    checked={choice.correct}
                    onChange={() => setCorrectChoice(i)}
                    className="accent-brand-blue"
                  />
                  <input
                    type="text"
                    value={choice.text}
                    onChange={(e) => setChoiceText(i, e.target.value)}
                    maxLength={MAX_CHOICE_LEN}
                    placeholder={`Choice ${i + 1}`}
                    required
                    className="flex-1 rounded-lg border border-gray-200 px-3 py-1.5 text-sm focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
                  />
                  {choices.length > MIN_CHOICES && (
                    <button type="button" onClick={() => removeChoice(i)}
                      className="rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-400 hover:bg-red-50 hover:text-brand-red">
                      ✕
                    </button>
                  )}
                </div>
              ))}
            </div>
            {choices.length < MAX_CHOICES && (
              <button type="button" onClick={addChoice}
                className="mt-2 text-xs font-medium text-brand-blue hover:underline">
                + Add choice
              </button>
            )}
          </div>
        )}

        <div className="flex justify-end gap-2 pt-1">
          <button type="button" onClick={onCancel} disabled={isPending}
            className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 hover:bg-slate-50 disabled:opacity-50">
            Cancel
          </button>
          <button type="submit" disabled={isPending}
            className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50">
            {isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}
