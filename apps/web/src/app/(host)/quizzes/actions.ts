'use server'

import { redirect } from 'next/navigation'
import { revalidatePath } from 'next/cache'
import * as quizzesApi from '@/lib/api/quizzes'
import * as gamesApi from '@/lib/api/games'
import { listQuestions } from '@/lib/api/questions'
import type { Question } from '@/types'

// listBankQuestionsAction lets the client-side picker fetch questions via a server action
// (keeping the auth token server-side).
export async function listBankQuestionsAction(bankID: string): Promise<Question[]> {
  try {
    return await listQuestions(bankID)
  } catch {
    return []
  }
}

export async function createQuizAction(formData: FormData): Promise<{ error?: string }> {
  const name = (formData.get('name') as string | null)?.trim()
  if (!name) return { error: 'Name is required' }

  let quiz
  try {
    quiz = await quizzesApi.createQuiz(name)
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to create quiz' }
  }

  redirect(`/quizzes/${quiz.id}`)
}

export async function deleteQuizAction(quizID: string): Promise<{ success?: boolean; error?: string }> {
  try {
    await quizzesApi.deleteQuiz(quizID)
    revalidatePath('/quizzes')
    return { success: true }
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to delete quiz' }
  }
}

export async function createRoundAction(quizID: string, title?: string): Promise<{ error?: string }> {
  try {
    await quizzesApi.createRound(quizID, title)
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to add round' }
  }
  return {}
}

export async function updateRoundAction(
  quizID: string, roundID: string, title: string
): Promise<{ error?: string }> {
  try {
    await quizzesApi.updateRound(quizID, roundID, title || undefined)
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to update round' }
  }
  return {}
}

export async function deleteRoundAction(quizID: string, roundID: string): Promise<{ error?: string }> {
  try {
    await quizzesApi.deleteRound(quizID, roundID)
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to delete round' }
  }
  return {}
}

export async function setRoundQuestionsAction(
  quizID: string, roundID: string, questionIDs: string[]
): Promise<{ error?: string }> {
  try {
    await quizzesApi.setRoundQuestions(quizID, roundID, questionIDs)
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to save questions' }
  }
  return {}
}

export async function createGameFromQuizAction(quizID: string): Promise<{ error?: string }> {
  let game
  try {
    game = await gamesApi.createGame({ quizID })
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'Failed to create game' }
  }
  redirect(`/games/${game.id}/host`)
}
