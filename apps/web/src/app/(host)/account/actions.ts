'use server'

import { revalidatePath } from 'next/cache'
import { updateMe } from '@/lib/api/user'
import type { UserProfile } from '@/types'

export type UpdateProfileResult =
  | { ok: true; profile: UserProfile }
  | { ok: false; error: string }

export async function updateProfile(
  _prev: UpdateProfileResult | null,
  formData: FormData,
): Promise<UpdateProfileResult> {
  const displayName = (formData.get('display_name') as string | null)?.trim() ?? ''

  if (displayName.length === 0 || displayName.length > 50) {
    return { ok: false, error: 'Display name must be 1–50 characters.' }
  }

  try {
    const profile = await updateMe({ display_name: displayName })
    revalidatePath('/account')
    return { ok: true, profile }
  } catch (err) {
    console.error('updateProfile action failed:', err)
    return { ok: false, error: 'Could not save changes. Please try again.' }
  }
}
