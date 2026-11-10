import type { BarterProduct } from '../data/barter'
import type { TradeVehicle } from './smuggler'

export interface BarterLoadoutItem extends BarterProduct {
  quantity: number
  estimatedValue: number
}

export interface BarterLoadout {
  items: BarterLoadoutItem[]
  slotsUsed: number
  totalWeight: number
  estimatedValue: number
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
  let remainingWeight = vehicle.weight
  const items: BarterLoadoutItem[] = []

  const rankedProducts = [...products].sort((a, b) =>
    b.level - a.level || stationPriority.indexOf(a.postId) - stationPriority.indexOf(b.postId) || a.id - b.id,
  )

  for (const product of rankedProducts) {
    if (items.length >= vehicle.block || product.weight <= 0 || product.boxCapacity <= 0) break
    const available = Math.min(product.weeklyLimit, product.boxCapacity, exchangeableCounts.get(product.id) || 0)
    const quantity = Math.min(available, Math.floor(remainingWeight / product.weight))
    if (quantity <= 0) continue

    items.push({
      ...product,
      quantity,
      estimatedValue: quantity * (estimatedValueByLevel[product.level] || 0),
    })
    remainingWeight -= quantity * product.weight
  }

  const totalWeight = items.reduce((sum, item) => sum + item.quantity * item.weight, 0)
  return {
    items,
    slotsUsed: items.length,
    totalWeight,
    estimatedValue: items.reduce((sum, item) => sum + item.estimatedValue, 0),
  }
}
