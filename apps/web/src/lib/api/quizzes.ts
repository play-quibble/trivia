// Server-side API client for quiz management.
// Called from server actions and server components only.

import type { Quiz, QuizDetail } from '@/types'

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
    let message = `API ${options.method ?? 'GET'} ${path} → ${res.status}`
    try {
      const json = JSON.parse(body)
      if (json?.error) message = json.error
    } catch {
      // not JSON — keep the raw body for debugging
      if (body) message = body
    }
    throw new Error(message)
  }

  return res
}

export async function listQuizzes(): Promise<Quiz[]> {
  const res = await apiFetch('/quizzes')
  return res.json()
}

export async function createQuiz(name: string, description?: string): Promise<Quiz> {
  const res = await apiFetch('/quizzes', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  })
  return res.json()
}

export async function getQuiz(id: string): Promise<QuizDetail> {
  const res = await apiFetch(`/quizzes/${id}`)
  return res.json()
}

export async function updateQuiz(id: string, name: string, description?: string): Promise<Quiz> {
  const res = await apiFetch(`/quizzes/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  })
  return res.json()
}

export async function deleteQuiz(id: string): Promise<void> {
  await apiFetch(`/quizzes/${id}`, { method: 'DELETE' })
}

export async function createRound(quizID: string, title?: string): Promise<void> {
  await apiFetch(`/quizzes/${quizID}/rounds`, {
    method: 'POST',
    body: JSON.stringify({ title }),
  })
}

export async function updateRound(quizID: string, roundID: string, title?: string): Promise<void> {
  await apiFetch(`/quizzes/${quizID}/rounds/${roundID}`, {
    method: 'PUT',
    body: JSON.stringify({ title }),
  })
}

export async function deleteRound(quizID: string, roundID: string): Promise<void> {
  await apiFetch(`/quizzes/${quizID}/rounds/${roundID}`, { method: 'DELETE' })
}

export async function setRoundQuestions(quizID: string, roundID: string, questionIDs: string[]): Promise<void> {
  await apiFetch(`/quizzes/${quizID}/rounds/${roundID}/questions`, {
    method: 'PUT',
    body: JSON.stringify({ question_ids: questionIDs }),
  })
}
