export function formatSelfName(name: string, masked: boolean): string {
  if (!masked || !name) return name
  const characters = Array.from(name)
  if (characters.length <= 1) return name
  return `${characters.slice(0, characters.length === 2 ? 1 : 2).join('')}**`
}
