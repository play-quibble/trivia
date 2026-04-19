// Games page — server component.
// Fetches the host's games and available banks, then renders the GamesView.
import { listGames } from '@/lib/api/games'
import { listBanks } from '@/lib/api/banks'
import GamesView from '@/components/GamesView'

export const metadata = { title: 'Games — Quibble' }

export default async function GamesPage() {
  const [games, banks] = await Promise.all([listGames(), listBanks()])
  return <GamesView games={games} banks={banks} />
}
