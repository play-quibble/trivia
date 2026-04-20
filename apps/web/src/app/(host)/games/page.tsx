// Games page — server component.
// Fetches the host's games and available quizzes, then renders the GamesView.
import { listGames } from '@/lib/api/games'
import { listQuizzes } from '@/lib/api/quizzes'
import GamesView from '@/components/GamesView'

export const metadata = { title: 'Games — Quibble' }

export default async function GamesPage() {
  const [games, quizzes] = await Promise.all([listGames(), listQuizzes()])
  return <GamesView games={games} quizzes={quizzes} />
}
