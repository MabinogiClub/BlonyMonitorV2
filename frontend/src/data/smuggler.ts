import products from '../assets/smuggler/product.json'
import vehicles from '../assets/smuggler/vehicle.json'
import { calculateVehicleLoads, type TradeProduct } from '../utils/smuggler'

// Game images and trade data from baibaibot's local smuggler query assets.
const areaImages = import.meta.glob<string>('../assets/smuggler/area/*.png', { eager: true, query: '?url', import: 'default' })
const productImages = import.meta.glob<string>('../assets/smuggler/product/*.png', { eager: true, query: '?url', import: 'default' })
const normalizeArea = (area: string) => area.replace(/\s+/g, '')
const areaImageByName = new Map(Object.entries(areaImages).map(([path, url]) => [
  normalizeArea(path.split('/').pop()!.replace(/\.png$/, '')), url,
]))

export function getSmugglerAreaImage(area: string): string | undefined {
  return areaImageByName.get(normalizeArea(area))
}

export function getSmugglerProduct(name: string): TradeProduct | undefined {
  return products.find(product => product.name === name)
}

export function getSmugglerProductImage(product: TradeProduct): string | undefined {
  return productImages[`../assets/smuggler/product/${product.img}`]
}

export function getSmugglerVehicles(product: TradeProduct) {
  return calculateVehicleLoads(product, vehicles)
}
