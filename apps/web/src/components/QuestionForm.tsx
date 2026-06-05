'use client'

import { useState } from 'react'
import type { MCChoice } from '@/types'
import {
  MAX_PROMPT_LEN,
  MAX_CHOICE_LEN,
  MAX_ANSWER_LEN,
  MAX_CHOICES,
  MIN_CHOICES,
  MAX_ANSWERS,
} from '@/types'

export type QuestionType = 'text' | 'multiple_choice'

// QuestionFormValues is the normalized payload handed to onSubmit:
// prompt is trimmed and acceptedAnswers are trimmed + emptied-filtered.
// The caller decides which API/action to call based on `type`.
export interface QuestionFormValues {
  type: QuestionType
  prompt: string
  points: number
  acceptedAnswers: string[]
  choices: MCChoice[]
}

interface QuestionFormProps {
  // Prefill for edit mode. Omitted fields fall back to empty defaults.
  initialValues?: Partial<QuestionFormValues>
  // Hide the text/MC toggle — used when editing a question whose type is fixed.
  lockType?: boolean
  // Optional heading rendered top-left (e.g. "New Question").
  title?: string
  submitLabel?: string
  pendingLabel?: string
  isPending?: boolean
  // Used both as the initial points value and the fallback when the field is cleared.
  defaultPoints?: number
  autoFocusPrompt?: boolean
  // Tailwind classes for the <form> element so each caller controls padding/spacing.
  className?: string
  onSubmit: (values: QuestionFormValues) => void
  onCancel: () => void
}

const emptyChoices = (): MCChoice[] => [
  { text: '', correct: true },
  { text: '', correct: false },
]

// QuestionForm is a purely presentational question editor. It owns prompt,
// points, accepted-answers and multiple-choice state plus the field helpers,
// but knows nothing about banks or server actions — on submit it hands a
// normalized QuestionFormValues to the caller. Used by AddQuestionForm (create),
// QuestionCard's edit form, and the inline picker in QuizBuilder.
export default function QuestionForm({
  initialValues,
  lockType = false,
  title,
  submitLabel = 'Add Question',
  pendingLabel = 'Saving…',
  isPending = false,
  defaultPoints = 1000,
  autoFocusPrompt = false,
  className = 'space-y-5',
  onSubmit,
  onCancel,
}: QuestionFormProps) {
  const [type, setType] = useState<QuestionType>(initialValues?.type ?? 'text')
  const [prompt, setPrompt] = useState(initialValues?.prompt ?? '')
  const [points, setPoints] = useState(String(initialValues?.points ?? defaultPoints))

  // Text question: list of accepted answers. First is always required.
  const [answers, setAnswers] = useState<string[]>(
    initialValues?.acceptedAnswers?.length ? initialValues.acceptedAnswers : [''],
  )

  // MC question: list of choices. Radio input marks the correct one.
  const [choices, setChoices] = useState<MCChoice[]>(
    initialValues?.choices?.length ? initialValues.choices : emptyChoices(),
  )

  // --- Text answer helpers ---
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

  // --- MC choice helpers ---
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
  // Only one choice can be correct at a time — selecting one deselects the rest.
  function setCorrectChoice(i: number) {
    setChoices(choices.map((c, idx) => ({ ...c, correct: idx === i })))
  }

  function handleTypeChange(next: QuestionType) {
    setType(next)
    // Reset answers/choices when switching types to avoid stale state.
    setAnswers([''])
    setChoices(emptyChoices())
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const pts = Math.max(1, parseInt(points, 10) || defaultPoints)
    onSubmit({
      type,
      prompt: prompt.trim(),
      points: pts,
      acceptedAnswers: answers.map((a) => a.trim()).filter(Boolean),
      choices,
    })
  }

  const showHeader = Boolean(title) || !lockType

  return (
    <form onSubmit={handleSubmit} className={className}>
      {showHeader && (
        <div className="flex items-center justify-between">
          {title ? (
            <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          ) : (
            <span />
          )}

          {/* Type toggle */}
          {!lockType && (
            <div className="flex rounded-lg border border-gray-200 p-0.5 text-sm">
              {(['text', 'multiple_choice'] as QuestionType[]).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => handleTypeChange(t)}
                  className={`rounded-md px-3 py-1 font-medium transition-colors ${
                    type === t
                      ? 'bg-brand-blue text-white'
                      : 'text-gray-500 hover:text-gray-700'
                  }`}
                >
                  {t === 'text' ? 'Text Answer' : 'Multiple Choice'}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

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
          autoFocus={autoFocusPrompt}
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          maxLength={MAX_PROMPT_LEN}
          rows={2}
          placeholder="e.g. What is the capital of France?"
          className="w-full resize-none rounded-lg border border-gray-200 px-3 py-2 text-sm placeholder-gray-400 focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
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

      {/* Text question: accepted answers */}
      {type === 'text' ? (
        <div>
          <p className="mb-2 text-sm font-medium text-gray-700">
            Accepted Answers <span className="text-brand-red">*</span>
            <span className="ml-2 font-normal text-xs text-gray-400">
              primary first — add alternates for synonyms or spelling variations
            </span>
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
                  className="flex-1 rounded-lg border border-gray-200 px-3 py-1.5 text-sm placeholder-gray-400 focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
                />
                {i > 0 && (
                  <button
                    type="button"
                    onClick={() => removeAnswer(i)}
                    className="rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-400 hover:bg-red-50 hover:text-brand-red"
                  >
                    ✕
                  </button>
                )}
              </div>
            ))}
          </div>
          {answers.length < MAX_ANSWERS && (
            <button
              type="button"
              onClick={addAnswer}
              className="mt-2 text-xs font-medium text-brand-blue hover:underline"
            >
              + Add alternate answer
            </button>
          )}
        </div>
      ) : (
        /* Multiple choice: choice inputs with radio to mark correct */
        <div>
          <p className="mb-2 text-sm font-medium text-gray-700">
            Choices <span className="text-brand-red">*</span>
            <span className="ml-2 font-normal text-xs text-gray-400">
              click the radio button to mark the correct answer
            </span>
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
                  aria-label={`Mark choice ${i + 1} as correct`}
                />
                <input
                  type="text"
                  value={choice.text}
                  onChange={(e) => setChoiceText(i, e.target.value)}
                  maxLength={MAX_CHOICE_LEN}
                  placeholder={`Choice ${i + 1}`}
                  required
                  className="flex-1 rounded-lg border border-gray-200 px-3 py-1.5 text-sm placeholder-gray-400 focus:border-brand-blue focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
                />
                {choices.length > MIN_CHOICES && (
                  <button
                    type="button"
                    onClick={() => removeChoice(i)}
                    className="rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-400 hover:bg-red-50 hover:text-brand-red"
                  >
                    ✕
                  </button>
                )}
              </div>
            ))}
          </div>
          {choices.length < MAX_CHOICES && (
            <button
              type="button"
              onClick={addChoice}
              className="mt-2 text-xs font-medium text-brand-blue hover:underline"
            >
              + Add choice
            </button>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <button
          type="button"
          onClick={onCancel}
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
          {isPending ? pendingLabel : submitLabel}
        </button>
      </div>
    </form>
  )
}
