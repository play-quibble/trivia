'use server'

import { revalidatePath } from 'next/cache'
import type { MCChoice, Question } from '@/types'
import {
  createTextQuestion,
  createMCQuestion,
  updateTextQuestion,
  updateMCQuestion,
  deleteQuestion,
  reorderQuestions,
} from '@/lib/api/questions'

// createTextQuestionAction creates a free-response question and returns it
// so the client can update local state immediately.
export async function createTextQuestionAction(
  bankId: string,
  prompt: string,
  acceptedAnswers: string[],
  points: number,
): Promise<{ question?: Question; error?: string }> {
  try {
    const question = await createTextQuestion(bankId, prompt, acceptedAnswers, points)
    revalidatePath(`/banks/${bankId}`)
    return { question }
  } catch (err) {
    console.error('createTextQuestionAction:', err)
    return { error: 'Failed to create question' }
  }
}

// createMCQuestionAction creates a multiple-choice question.
export async function createMCQuestionAction(
  bankId: string,
  prompt: string,
  choices: MCChoice[],
  points: number,
): Promise<{ question?: Question; error?: string }> {
  try {
    const question = await createMCQuestion(bankId, prompt, choices, points)
    revalidatePath(`/banks/${bankId}`)
    return { question }
  } catch (err) {
    console.error('createMCQuestionAction:', err)
    return { error: 'Failed to create question' }
  }
}

// updateTextQuestionAction updates a free-response question.
export async function updateTextQuestionAction(
  bankId: string,
  questionId: string,
  prompt: string,
  acceptedAnswers: string[],
  points: number,
): Promise<{ question?: Question; error?: string }> {
  try {
    const question = await updateTextQuestion(bankId, questionId, prompt, acceptedAnswers, points)
    revalidatePath(`/banks/${bankId}`)
    return { question }
  } catch (err) {
    console.error('updateTextQuestionAction:', err)
    return { error: 'Failed to update question' }
  }
}

// updateMCQuestionAction updates a multiple-choice question.
export async function updateMCQuestionAction(
  bankId: string,
  questionId: string,
  prompt: string,
  choices: MCChoice[],
  points: number,
): Promise<{ question?: Question; error?: string }> {
  try {
    const question = await updateMCQuestion(bankId, questionId, prompt, choices, points)
    revalidatePath(`/banks/${bankId}`)
    return { question }
  } catch (err) {
    console.error('updateMCQuestionAction:', err)
    return { error: 'Failed to update question' }
  }
}

// deleteQuestionAction permanently removes a question.
export async function deleteQuestionAction(
  bankId: string,
  questionId: string,
): Promise<{ success: boolean; error?: string }> {
  try {
    await deleteQuestion(bankId, questionId)
    revalidatePath(`/banks/${bankId}`)
    return { success: true }
  } catch (err) {
    console.error('deleteQuestionAction:', err)
    return { success: false, error: 'Failed to delete question' }
  }
}

// reorderQuestionsAction persists a new question order.
// The client calls this after moving a question up or down.
export async function reorderQuestionsAction(
  bankId: string,
  orderedIds: string[],
): Promise<{ success: boolean; error?: string }> {
  try {
    await reorderQuestions(bankId, orderedIds)
    revalidatePath(`/banks/${bankId}`)
    return { success: true }
  } catch (err) {
    console.error('reorderQuestionsAction:', err)
    return { success: false, error: 'Failed to reorder questions' }
  }
}
