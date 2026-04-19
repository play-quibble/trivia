// Player game page.
// The session token lives in sessionStorage (set by /join) so this is a
// client-side-only page — no server fetching needed.
import PlayerGame from '@/components/PlayerGame'

interface Props {
  params: Promise<{ code: string }>
}

export async function generateMetadata({ params }: Props) {
  const { code } = await params
  return { title: `Playing ${code} — Quibble` }
}

export default async function PlayPage({ params }: Props) {
  const { code } = await params
  // API URL must be public so the browser can reach the WebSocket endpoint.
  const wsBase = (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080')
    .replace(/^http/, 'ws')
    .replace(/\/$/, '')

  return <PlayerGame code={code.toUpperCase()} wsBase={wsBase} />
}
