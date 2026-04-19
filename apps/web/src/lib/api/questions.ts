// API client for questions within a bank.
// All functions run server-side only (called from server actions / server components).

import type { Question, MCChoice } from '@/types'

const API_BASE = (process.env.API_URL ?? 'http://localhost:8080').replace(/\/$/, '')
const DEV_TOKEN = process.env.DEV_AUTH_TOKEN ?? ''

async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const url = `${API_BASE}${path}`
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${DEV_TOKEN}`,
      ...options.headers,
    },
    cache: 'no-store',
  })
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`API ${options.method ?? 'GET'} ${path} → ${res.status}: ${body}`)
  }
  return res
}

// listQuestions returns all questions for a bank, ordered by position.
export async function listQuestions(bankId: string): Promise<Question[]> {
  const res = await apiFetch(`/banks/${bankId}/questions`)
  return res.json()
}

// createTextQuestion creates a text (free-response) question.
export async function createTextQuestion(
  bankId: string,
  prompt: string,
  acceptedAnswers: string[],
  points: number,
): Promise<Question> {
  const res = await apiFetch(`/banks/${bankId}/questions`, {
    method: 'POST',
    body: JSON.stringify({ type: 'text', prompt, accepted_answers: acceptedAnswers, points }),
  })
  return res.json()
}

// createMCQuestion creates a multiple-choice question.
export async function createMCQuestion(
  bankId: string,
  prompt: string,
  choices: MCChoice[],
  points: number,
): Promise<Question> {
  const res = await apiFetch(`/banks/${bankId}/questions`, {
    method: 'POST',
    body: JSON.stringify({ type: 'multiple_choice', prompt, choices, points }),
  })
  return res.json()
}

// updateTextQuestion updates a text question's content.
export async function updateTextQuestion(
  bankId: string,
  questionId: string,
  prompt: string,
  acceptedAnswers: string[],
  points: number,
): Promise<Question> {
  const res = await apiFetch(`/banks/${bankId}/questions/${questionId}`, {
    method: 'PUT',
    body: JSON.stringify({ prompt, accepted_answers: acceptedAnswers, points }),
  })
  return res.json()
}

// updateMCQuestion updates a multiple-choice question's content.
export async function updateMCQuestion(
  bankId: string,
  questionId: string,
  prompt: string,
  choices: MCChoice[],
  points: number,
): Promise<Question> {
  const res = await apiFetch(`/banks/${bankId}/questions/${questionId}`, {
    method: 'PUT',
    body: JSON.stringify({ prompt, choices, points }),
  })
  return res.json()
}

// deleteQuestion permanently removes a question.
export async function deleteQuestion(bankId: string, questionId: string): Promise<void> {
  await apiFetch(`/banks/${bankId}/questions/${questionId}`, { method: 'DELETE' })
}

// reorderQuestions sends the full ordered list of question IDs to the API,
// which updates each question's position to match its index in the array.
export async function reorderQuestions(bankId: string, orderedIds: string[]): Promise<void> {
  await apiFetch(`/banks/${bankId}/questions/reorder`, {
    method: 'PATCH',
    body: JSON.stringify({ ids: orderedIds }),
  })
}
