import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'

// Compile the two pure helpers in memory so the tests also run on Node 20.
function moduleURL(source) {
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.ESNext },
  })
  return `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
}
const eirinURL = moduleURL(readFileSync(new URL('../src/utils/eirinTime.ts', import.meta.url), 'utf8'))
const source = readFileSync(new URL('../src/utils/smuggler.ts', import.meta.url), 'utf8')
const { getSmugglerWindow, isCurrentSmugglerNotice, getSmugglerMarqueeText, calculateVehicleLoads } = await import(moduleURL(source.replace("'./eirinTime'", JSON.stringify(eirinURL))))
const { getEirinDayStart, EIRIN_DAY_MS } = await import(eirinURL)
const offset = -14
const dayStart = getEirinDayStart(Date.parse('2026-11-09T20:00:00+08:00'), offset)
const at = minute => dayStart + minute * 1500
const notice = (type, minute, item = '热气结晶') => ({ type, receivedAt: at(minute), area: '斯利比 岩石地带东侧', item })

test('19:00, 22:00, 03:00 and 06:00 switch at the exact calibrated boundaries', () => {
  for (const [minute, before, after] of [
    [1140, 'absent', 'forecast'], [1320, 'forecast', 'appear'],
    [1620, 'appear', 'disappear_forecast'], [1800, 'disappear_forecast', 'absent'],
  ]) {
    assert.equal(getSmugglerWindow(at(minute) - 1, offset).phase, before)
    assert.equal(getSmugglerWindow(at(minute), offset).phase, after)
  }
})

test('the appearance window survives Eirin midnight and uses real-time durations', () => {
  const evening = getSmugglerWindow(at(1140), offset)
  const midnight = getSmugglerWindow(at(1440), offset)
  assert.equal(midnight.forecastAt, evening.forecastAt)
  assert.equal(evening.appearAt - evening.forecastAt, 4.5 * 60000)
  assert.equal(evening.goneAt - evening.appearAt, 12 * 60000)
  assert.equal(evening.goneAt - at(1620), 4.5 * 60000)
})

test('all three announcement types identify the same appearance, including a missed forecast', () => {
  for (const packet of [notice('forecast', 1140), notice('appear', 1320), notice('disappear_forecast', 1620, '')]) {
    assert.equal(isCurrentSmugglerNotice(packet, at(1650), offset), true)
  }
  assert.equal(isCurrentSmugglerNotice(null, at(1650), offset), false)
  assert.equal(isCurrentSmugglerNotice(notice('appear', 1320), at(1200), offset), false)
})

test('marquee requires trade goods and keeps goods carried from the earlier notice', () => {
  assert.equal(getSmugglerMarqueeText(notice('disappear_forecast', 1620, ''), '即将消失', '后消失'), '')
  assert.match(getSmugglerMarqueeText(notice('disappear_forecast', 1620), '即将消失', '后消失'), /热气结晶/)
})

test('location and goods expire at 06:00 and never carry into the following cycle', () => {
  const packet = notice('forecast', 1140)
  assert.equal(isCurrentSmugglerNotice(packet, at(1800) - 1, offset), true)
  assert.equal(isCurrentSmugglerNotice(packet, at(1800), offset), false)
  assert.equal(isCurrentSmugglerNotice(packet, at(1140) + EIRIN_DAY_MS, offset), false)
  assert.equal(isCurrentSmugglerNotice(packet, at(1650) + EIRIN_DAY_MS, offset), false)
})

test('homepage calibration moves event times while retaining the current announcement', () => {
  const now = at(1500)
  const original = getSmugglerWindow(now, offset)
  const adjusted = getSmugglerWindow(now, offset + 1)
  assert.equal(adjusted.goneAt, original.goneAt - 1500)
  assert.equal(isCurrentSmugglerNotice(notice('appear', 1320), now, offset + 1), true)
})

const products = JSON.parse(readFileSync(new URL('../src/assets/smuggler/product.json', import.meta.url)))
const vehicles = JSON.parse(readFileSync(new URL('../src/assets/smuggler/vehicle.json', import.meta.url)))

test('hot air crystal matches the reference loading and purchase costs', () => {
  const product = products.find(product => product.name === '热气结晶')
  const rows = calculateVehicleLoads(product, vehicles)
  assert.equal(rows[0].name, '[伙伴]迅捷的雪橇狗')
  assert.equal(rows[0].totalItems, 429)
  assert.equal(rows[0].actualSlots, 13)
  assert.equal(rows[0].totalCost, 4719)
  assert.equal(rows[1].totalItems, 396)
})

test('weight limits, partial slots and speed tie breaking apply across every local product', () => {
  for (const product of products) {
    const rows = calculateVehicleLoads(product, vehicles)
    for (const row of rows) {
      assert(row.totalItems * product.weight <= row.weight)
      assert(row.totalItems <= row.block * product.boxCount)
      assert(row.actualSlots <= row.block)
      assert(row.actualSlots === 0 || (row.actualSlots - 1) * product.boxCount < row.totalItems)
    }
    for (let i = 1; i < rows.length; i++) {
      assert(rows[i - 1].totalCost >= rows[i].totalCost)
      if (rows[i - 1].totalCost === rows[i].totalCost) assert(rows[i - 1].speed >= rows[i].speed)
    }
  }
})

const barterSource = readFileSync(new URL('../src/utils/barterLoadout.ts', import.meta.url), 'utf8')
const { calculateBarterLoadout } = await import(moduleURL(barterSource))
const barterProducts = (await import(moduleURL(readFileSync(new URL('../src/data/barter.ts', import.meta.url), 'utf8')))).BARTER_PRODUCTS

test('the partner airship maximizes estimated total value within stack, slot and weight limits', () => {
  const exchangeable = new Map(barterProducts.map(product => [product.id, product.weeklyLimit]))
  const ship = { name: 'airship', block: 10, weight: 1500, speed: 240 }
  const loadout = calculateBarterLoadout(barterProducts, exchangeable, ship)

  assert.equal(loadout.slotsUsed, 10)
  assert.equal(loadout.slotsUsed, 10)
  assert(loadout.items.every(item => item.quantity <= item.boxCapacity))
  assert.equal(loadout.totalValue, 3017933)
  assert(loadout.totalWeight <= ship.weight)
  assert(loadout.slotsUsed <= ship.block)
})

test('loadout respects backpack availability and can use partial stacks on a small vehicle', () => {
  const firstProduct = barterProducts.find(product => product.id === 20405)
  const exchangeable = new Map([[firstProduct.id, 20]])
  const smallVehicle = { name: 'small', block: 1, weight: 50, speed: 100 }
  const loadout = calculateBarterLoadout(barterProducts, exchangeable, smallVehicle)

  assert.equal(loadout.slotsUsed, 1)
  assert.equal(loadout.totalWeight, 30)
  assert.equal(loadout.items[0].quantity, 1)
  assert.equal(loadout.totalValue, 150000)
})
