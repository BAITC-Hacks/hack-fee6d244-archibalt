/** Быстрые варианты значения для текстового поля. */
export function Chips({ label, options, value, onPick }: { label: string; options: string[]; value?: string; onPick: (value: string) => void }) {
  if (!options.length) return null
  return <ul className="ui-chips" aria-label={label}>
    {options.map(option => <li key={option}><button type="button" className="ui-chip" aria-pressed={value === option} onClick={() => onPick(option)}>{option}</button></li>)}
  </ul>
}
