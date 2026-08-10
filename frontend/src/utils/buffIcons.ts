export function getBuffIconUrl(buffId: number): string {
  return `${import.meta.env.BASE_URL}buff-icons/${buffId}.png`
}
