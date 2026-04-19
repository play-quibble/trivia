'use server'

// Server actions for question bank mutations.
// Switched from the in-memory mock store to the real Go API — all changes
// now persist to Postgres via the Go API's /banks endpoints.

import { revalidatePath } from 'next/cache'
import * as banksApi from '@/lib/api/banks'
import type { Bank } from '@/types'

export async function createBankAction(
  formData: FormData,
): Promise<{ bank?: Bank; error?: string }> {
  const name = (formData.get('name') as string | null)?.trim()
  const description = (formData.get('description') as string | null)?.trim() || undefined

  if (!name) return { error: 'Name is required' }

  try {
    const bank = await banksApi.createBank(name, description)
    revalidatePath('/banks')
    return { bank }
  } catch (err) {
    console.error('createBankAction failed:', err)
    return { error: 'Failed to create bank' }
  }
}

export async function updateBankAction(
  bankId: string,
  formData: FormData,
): Promise<{ bank?: Bank; error?: string }> {
  const name = (formData.get('name') as string | null)?.trim()
  const description = (formData.get('description') as string | null)?.trim() || undefined

  if (!name) return { error: 'Name is required' }

  try {
    const bank = await banksApi.updateBank(bankId, name, description)
    revalidatePath('/banks')
    return { bank }
  } catch (err) {
    console.error('updateBankAction failed:', err)
    return { error: 'Failed to update bank' }
  }
}

export async function deleteBankAction(
  bankId: string,
): Promise<{ success: boolean; error?: string }> {
  try {
    await banksApi.deleteBank(bankId)
    revalidatePath('/banks')
    return { success: true }
  } catch (err) {
    console.error('deleteBankAction failed:', err)
    return { success: false, error: 'Failed to delete bank' }
  }
}
