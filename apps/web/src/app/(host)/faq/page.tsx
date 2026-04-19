export const metadata = { title: 'FAQ — Quibble' }

const faqs = [
  {
    q: 'What is Quibble?',
    a: 'Quibble is a self-hosted live trivia platform. Hosts create question banks and run games; players join instantly via QR code with no account required.',
  },
  {
    q: 'Do players need to create an account?',
    a: 'Nope. Players join using a 6-character game code (or by scanning a QR code) and pick a display name. No sign-up, no password.',
  },
  {
    q: 'What is a question bank?',
    a: 'A question bank is a reusable set of questions a host builds ahead of time. When you launch a game you pick a bank, and Quibble works through it question by question.',
  },
  {
    q: 'What question types are supported?',
    a: 'Text (free-response) and multiple choice. Each question has a point value you set when building the bank.',
  },
  {
    q: 'Who can host a game?',
    a: 'Anyone with a Quibble account. Hosts log in, create question banks, and launch games from the dashboard.',
  },
  {
    q: 'How does real-time scoring work?',
    a: 'Live game state — scores, answers, and the active question — is stored in Redis and pushed to all connected players over WebSocket. Final scores are flushed to the database when the game ends.',
  },
  {
    q: 'What does the infrastructure look like?',
    a: 'Quibble runs on Kubernetes, initially on DigitalOcean Kubernetes (DOKS) with a clear migration path to Google Kubernetes Engine (GKE). GitOps is handled by Argo CD.',
  },
  {
    q: <>What does the <em>bb</em> stand for?</>,
    a: (
      <>
        <a
          href="https://www.linkedin.com/in/ben-botsford/"
          target="_blank"
          rel="noopener noreferrer"
          className="text-brand-blue underline underline-offset-2 hover:opacity-75"
        >
          None of your business!
        </a>
      </>
    ),
  },
]

export default function FAQPage() {
  return (
    <main className="mx-auto max-w-2xl px-6 py-10">
      <h1 className="mb-8 text-2xl font-semibold text-gray-900">
        Frequently Asked Questions
      </h1>

      <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
        <dl className="divide-y divide-gray-100">
          {faqs.map((faq, i) => (
            <div key={i} className="px-7 py-6">
              <dt className="text-base font-semibold text-gray-900">{faq.q}</dt>
              <dd className="mt-2 text-sm leading-relaxed text-gray-500">{faq.a}</dd>
            </div>
          ))}
        </dl>
      </div>
    </main>
  )
}
