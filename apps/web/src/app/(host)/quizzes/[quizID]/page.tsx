import { getQuiz } from '@/lib/api/quizzes'
import { listBanks } from '@/lib/api/banks'
import QuizBuilder from '@/components/QuizBuilder'

interface Props {
  params: Promise<{ quizID: string }>
}

export default async function QuizPage({ params }: Props) {
  const { quizID } = await params

  const [quiz, banks] = await Promise.all([
    getQuiz(quizID),
    listBanks().catch(() => []),
  ])

  return <QuizBuilder quiz={quiz} banks={banks} />
}
