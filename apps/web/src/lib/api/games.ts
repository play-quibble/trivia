// Real API client for games.
// Server-side only — called from server actions and server components.
// The DEV_AUTH_TOKEN is never exposed to the browser.

import type { Game, GamePlayer } from '@/types'

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

// listGames returns all games created by the authenticated host, newest first.
export async function listGames(): Promise<Game[]> {
  const res = await apiFetch('/games')
  return res.json()
}

// createGame creates a new game linked to a question bank.
export async function createGame(bankID: string, roundSize: number): Promise<Game> {
  const res = await apiFetch('/games', {
    method: 'POST',
    body: JSON.stringify({ bank_id: bankID, round_size: roundSize }),
  })
  return res.json()
}

// getGame fetches a single game by ID.
export async function getGame(id: string): Promise<Game> {
  const res = await apiFetch(`/games/${id}`)
  return res.json()
}

// listPlayers returns current players in a game (for the lobby UI).
export async function listPlayers(gameID: string): Promise<GamePlayer[]> {
  const res = await apiFetch(`/games/${gameID}/players`)
  return res.json()
}
