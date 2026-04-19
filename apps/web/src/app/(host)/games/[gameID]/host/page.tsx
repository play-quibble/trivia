// Host game panel — server component.
// Fetches game data server-side and passes the WebSocket config
// to the HostGame client component.
import { notFound } from 'next/navigation'
import { getGame, listPlayers } from '@/lib/api/games'
import HostGame from '@/components/HostGame'

interface Props {
  params: Promise<{ gameID: string }>
}

export async function generateMetadata({ params }: Props) {
  const { gameID } = await params
  try {
    const game = await getGame(gameID)
    return { title: `Host — ${game.code} — Quibble` }
  } catch {
    return { title: 'Host Panel — Quibble' }
  }
}

export default async function HostGamePage({ params }: Props) {
  const { gameID } = await params

  let game, players
  try {
    ;[game, players] = await Promise.all([getGame(gameID), listPlayers(gameID)])
  } catch {
    notFound()
  }

  // Read server-only env vars and pass them down as props.
  // The client component needs the WS URL and host token to connect.
  const wsBase = (process.env.API_URL ?? 'http://localhost:8080')
    .replace(/^http/, 'ws')
    .replace(/\/$/, '')
  const hostToken = process.env.DEV_AUTH_TOKEN ?? ''

  return (
    <HostGame
      game={game}
      initialPlayers={players}
      wsBase={wsBase}
      hostToken={hostToken}
    />
  )
}
