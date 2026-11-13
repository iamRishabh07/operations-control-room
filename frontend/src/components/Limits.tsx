import { Assessment } from '../api'

export function Limits({ a }: { a: Assessment }) {
  if (!a.limitations || a.limitations.length === 0) return null
  return (
    <section className="limits">
      <h2>Known limits</h2>
      <ul>
        {a.limitations.map((l) => (
          <li key={l}>{l}</li>
        ))}
      </ul>
    </section>
  )
}
