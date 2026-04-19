import Link from 'next/link'
import { notFound } from 'next/navigation'
import { getBank } from '@/lib/api/banks'
import { listQuestions } from '@/lib/api/questions'
import QuestionsView from '@/components/QuestionsView'

export async function generateMetadata({ params }: { params: Promise<{ bankID: string }> }) {
  const { bankID } = await params
  try {
    const bank = await getBank(bankID)
    return { title: `${bank.name} — Quibble` }
  } catch {
    return { title: 'Bank — Quibble' }
  }
}

export default async function BankDetailPage({
  params,
}: {
  params: Promise<{ bankID: string }>
}) {
  const { bankID } = await params

  // Fetch bank and questions in parallel — both are needed before render.
  let bank, questions
  try {
    ;[bank, questions] = await Promise.all([getBank(bankID), listQuestions(bankID)])
  } catch {
    notFound()
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-10">
      {/* Breadcrumb */}
      <div className="mb-6">
        <Link
          href="/banks"
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          ← Question Banks
        </Link>
      </div>

      {/* Bank header */}
      <div className="mb-8">
        <h1 className="text-2xl font-semibold text-gray-900">{bank!.name}</h1>
        {bank!.description && (
          <p className="mt-1 text-sm text-gray-500">{bank!.description}</p>
        )}
        <p className="mt-1 text-xs text-gray-400">
          {questions!.length === 0
            ? 'No questions yet'
            : `${questions!.length} question${questions!.length === 1 ? '' : 's'}`}
        </p>
      </div>

      <QuestionsView bankId={bankID} initialQuestions={questions!} />
    </main>
  )
}
