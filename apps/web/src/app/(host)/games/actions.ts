'use server'

import { redirect } from 'next/navigation'
import { revalidatePath } from 'next/cache'
import * as gamesApi from '@/lib/api/games'
import type { Game } from '@/types'

// createGameAction creates a new game and redirects the host to the host panel.
export async function createGameAction(
  formData: FormData,
): Promise<{ error?: string }> {
  const bankID = formData.get('bank_id') as string | null
  const roundSizeRaw = formData.get('round_size') as string | null
  const roundSize = parseInt(roundSizeRaw ?? '5', 10)

  if (!bankID) return { error: 'A question bank is required' }
  if (isNaN(roundSize) || roundSize < 1) return { error: 'Round size must be at least 1' }

  let game: Game
  try {
    game = await gamesApi.createGame(bankID, roundSize)
  } catch (err) {
    console.error('createGameAction failed:', err)
    return { error: 'Failed to create game — make sure the bank has at least one question' }
  }

  revalidatePath('/games')
  redirect(`/games/${game.id}/host`)
}
