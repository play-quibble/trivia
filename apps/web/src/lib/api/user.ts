// Server-side API client for the /me endpoints.
// Never imported in client components — the DEV_AUTH_TOKEN must stay server-only.

import type { UserProfile } from '@/types'

const API_BASE = (process.env.API_URL ?? 'http://localhost:8080').replace(/\/$/, '')
const DEV_TOKEN = process.env.DEV_AUTH_TOKEN ?? ''

async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const res = await fetch(`${API_BASE}${path}`, {
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

// getMe fetches the authenticated host's profile.
export async function getMe(): Promise<UserProfile> {
  return (await apiFetch('/me')).json()
}

// updateMe patches display_name and/or email.
// Only the fields present in `data` are changed; omitted fields are preserved.
export async function updateMe(data: {
  display_name?: string
  email?: string
}): Promise<UserProfile> {
  return (await apiFetch('/me', {
    method: 'PATCH',
    body: JSON.stringify(data),
  })).json()
}
