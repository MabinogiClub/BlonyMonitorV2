import type { BarterProduct } from '../data/barter'
import type { TradeVehicle } from './smuggler'

export interface BarterLoadoutItem extends BarterProduct {
  quantity: number
}

export interface BarterLoadout {
  items: BarterLoadoutItem[]
  slotsUsed: number
  totalWeight: number
  totalValue: number
}

const stationPriority = [204, 203, 202, 201]
const estimatedValueByLevel: Record<number, number> = {
  1: 4000,
  2: 8500,
  3: 17000,
  4: 35000,
  5: 150000,
}

export function calculateBarterLoadout(
  products: readonly BarterProduct[],
  exchangeableCounts: ReadonlyMap<number, number>,
  vehicle: TradeVehicle,
): BarterLoadout {
  const rankedProducts = [...products].sort((a, b) =>
    b.level - a.level || stationPriority.indexOf(a.postId) - stationPriority.indexOf(b.postId) || a.id - b.id,
  )
  const stationValueOffset = new Map(stationPriority.map((stationId, index) => [stationId, index]))
  let states = new Map<string, { value: number; picks: number[] }>([['0:0', { value: 0, picks: [] }]])

  for (const product of rankedProducts) {
    const available = Math.min(product.weeklyLimit, exchangeableCounts.get(product.id) || 0)
    const unitValue = (estimatedValueByLevel[product.level] || 0) - (stationValueOffset.get(product.postId) ?? 0)
    const maxQuantity = product.weight > 0 && product.boxCapacity > 0
      ? Math.min(available, vehicle.block * product.boxCapacity, Math.floor(vehicle.weight / product.weight))
      : 0
    const nextStates = new Map<string, { value: number; picks: number[] }>()

    for (const [key, state] of states) {
      const [usedSlots, usedWeight] = key.split(':').map(Number)
      for (let quantity = 0; quantity <= maxQuantity; quantity++) {
        const slots = quantity === 0 ? 0 : Math.ceil(quantity / product.boxCapacity)
        const weight = quantity * product.weight
        const totalSlots = usedSlots + slots
        const totalWeight = usedWeight + weight
        if (totalSlots > vehicle.block || totalWeight > vehicle.weight) continue

        const nextKey = `${totalSlots}:${totalWeight}`
        const candidate = { value: state.value + quantity * unitValue, picks: [...state.picks, quantity] }
        const current = nextStates.get(nextKey)
        if (!current || candidate.value > current.value) nextStates.set(nextKey, candidate)
      }
    }
    states = nextStates
  }

  const best = [...states.values()].reduce((winner, state) => state.value > winner.value ? state : winner)
  const items: BarterLoadoutItem[] = []
  rankedProducts.forEach((product, index) => {
    let remaining = best.picks[index] || 0
    while (remaining > 0) {
      const quantity = Math.min(remaining, product.boxCapacity)
      items.push({ ...product, quantity })
      remaining -= quantity
    }
  })

  const totalWeight = items.reduce((sum, item) => sum + item.quantity * item.weight, 0)
  return {
    items,
    slotsUsed: items.length,
    totalWeight,
    totalValue: best.value,
  }
}
