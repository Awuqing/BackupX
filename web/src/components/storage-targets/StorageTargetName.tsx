import { IconStarFill } from '../icons'

interface StorageTargetNameProps {
  name: string
  starred?: boolean
}

export function StorageTargetName({ name, starred = false }: StorageTargetNameProps) {
  return (
    <span
      aria-label={starred ? `${name}，已收藏` : undefined}
      style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}
    >
      {starred ? <IconStarFill /> : null}
      <span>{name}</span>
    </span>
  )
}
