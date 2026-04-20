'use server'

import { redirect } from 'next/navigation'
import { revalidatePath } from 'next/cache'
import * as gamesApi from '@/lib/api/games'
import type { Game } from '@/types'

// createGameAction creates a new game linked to a quiz and redirects the host to the host panel.
export async function createGameAction(
  formData: FormData,
): Promise<{ error?: string }> {
  const quizID = formData.get('quiz_id') as string | null

  if (!quizID) return { error: 'A quiz is required' }

  let game: Game
  try {
    game = await gamesApi.createGame({ quizID })
  } catch (err) {
    console.error('createGameAction failed:', err)
    return { error: 'Failed to create game — make sure the quiz has at least one round with questions' }
  }

  revalidatePath('/games')
  redirect(`/games/${game.id}/host`)
}
