// Real API client for question banks.
//
// This module replaces src/lib/mock/banks.ts. It calls the Go API over HTTP,
// sending DEV_AUTH_TOKEN as a Bearer token so the API's dev bypass accepts
// the request without a real Auth0 JWT.
//
// These functions run exclusively on the server (called from server actions and
// server components), so the token and API URL are never exposed to the browser.
// There is no NEXT_PUBLIC_ prefix — they are server-only env vars.

import type { Bank } from '@/types'

const API_BASE = (process.env.API_URL ?? 'http://localhost:8080').replace(/\/$/, '')
const DEV_TOKEN = process.env.DEV_AUTH_TOKEN ?? ''

// apiFetch is a thin wrapper around fetch that:
//  - Prefixes the path with the Go API base URL
//  - Attaches the Authorization header
//  - Sets cache: 'no-store' so Next.js never serves a stale API response
//  - Throws a descriptive error on non-2xx responses
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
    // Read the error body (the Go API returns {"error":"..."} on failures)
    // and include it in the thrown error so it surfaces in server logs.
    const body = await res.text().catch(() => '')
    throw new Error(`API ${options.method ?? 'GET'} ${path} → ${res.status}: ${body}`)
  }

  return res
}

// listBanks fetches all banks owned by the authenticated user.
// The API derives the owner from the Bearer token — no user ID needed here.
export async function listBanks(): Promise<Bank[]> {
  const res = await apiFetch('/banks')
  return res.json()
}

// createBank creates a new bank and returns the created record.
export async function createBank(name: string, description?: string): Promise<Bank> {
  const res = await apiFetch('/banks', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  })
  return res.json()
}

// updateBank replaces a bank's name and description, returning the updated record.
export async function updateBank(id: string, name: string, description?: string): Promise<Bank> {
  const res = await apiFetch(`/banks/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  })
  return res.json()
}

// deleteBank permanently removes a bank.
// The Go API returns 204 No Content on success — no body to parse.
export async function deleteBank(id: string): Promise<void> {
  await apiFetch(`/banks/${id}`, { method: 'DELETE' })
}
