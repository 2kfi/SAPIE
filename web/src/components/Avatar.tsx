type Props = {
  name: string
  small?: boolean
}

function initial(name: string): string {
  const first = Array.from(name.trim())[0]
  return first ? first.toUpperCase() : '?'
}

export default function Avatar({ name, small = false }: Props) {
  return (
    <span className={`avatar${small ? ' avatar--sm' : ''}`} aria-hidden="true">
      {initial(name)}
    </span>
  )
}
