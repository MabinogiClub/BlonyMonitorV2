export interface BarterMaterial {
  itemId: number
  amount: number
}

export interface BarterProduct {
  id: number
  postId: number
  level: number
  name: string
  weeklyLimit: number
  boxCapacity: number
  weight: number
  materials: BarterMaterial[]
}

export const BARTER_STATIONS = [
  { id: 204, name: '佩拉交易所' },
  { id: 203, name: '凯利达交易所' },
  { id: 202, name: '绿洲交易所' },
  { id: 201, name: '卡鲁森林交易所' },
]

// R147 ItemDB sac_item fields: material bags carry their contained quantity.
export const BARTER_MATERIAL_BAGS: Record<number, readonly number[]> = {
  60011: [2052, 2122, 5110022, 5110039],
  60012: [2053, 2123, 5110023, 5110040],
  60013: [2054, 2124, 5110024, 5110041],
  60014: [2055, 2125, 5110025, 5110042],
  60019: [2048, 2118, 5110018, 5110035],
  60020: [2049, 2119, 5110019, 5110036],
  60021: [2050, 2120, 5110020, 5110037],
  60022: [2051, 2121, 5110021, 5110038],
}

export function addBarterMaterialBagContents(counts: Map<number, number>): Map<number, number> {
  const result = new Map(counts)
  for (const [materialId, bagIds] of Object.entries(BARTER_MATERIAL_BAGS)) {
    const containedAmount = bagIds.reduce((sum, bagId) => sum + (counts.get(bagId) || 0), 0)
    if (containedAmount > 0) result.set(Number(materialId), (result.get(Number(materialId)) || 0) + containedAmount)
  }
  return result
}

// R147 CommerceCommon, gf2022CommerceAlbanEiler recipes.
export const BARTER_PRODUCTS: BarterProduct[] = [
  { id: 20101, postId: 201, level: 1, name: '木制桌', weeklyLimit: 25, boxCapacity: 10, weight: 15, materials: [{ itemId: 50664, amount: 4 }, { itemId: 67201, amount: 2 }] },
  { id: 20102, postId: 201, level: 2, name: '木工艺品', weeklyLimit: 15, boxCapacity: 7, weight: 15, materials: [{ itemId: 5100003, amount: 1 }, { itemId: 60051, amount: 2 }] },
  { id: 20103, postId: 201, level: 3, name: '石像马雕像', weeklyLimit: 10, boxCapacity: 7, weight: 20, materials: [{ itemId: 67110, amount: 2 }, { itemId: 50022, amount: 5 }] },
  { id: 20104, postId: 201, level: 4, name: '卡鲁香菇', weeklyLimit: 8, boxCapacity: 7, weight: 25, materials: [{ itemId: 67111, amount: 1 }, { itemId: 63219, amount: 5 }, { itemId: 60013, amount: 4 }] },
  { id: 20105, postId: 201, level: 5, name: '贝壳化石', weeklyLimit: 3, boxCapacity: 5, weight: 30, materials: [{ itemId: 67114, amount: 2 }, { itemId: 5100058, amount: 1 }, { itemId: 5100002, amount: 5 }] },
  { id: 20201, postId: 202, level: 1, name: '细沙', weeklyLimit: 25, boxCapacity: 10, weight: 15, materials: [{ itemId: 51172, amount: 3 }, { itemId: 60004, amount: 2 }] },
  { id: 20202, postId: 202, level: 2, name: '冰霜幽灵的翅膀', weeklyLimit: 15, boxCapacity: 7, weight: 15, materials: [{ itemId: 64303, amount: 1 }, { itemId: 60014, amount: 2 }] },
  { id: 20203, postId: 202, level: 3, name: '绿洲图画', weeklyLimit: 10, boxCapacity: 7, weight: 20, materials: [{ itemId: 60030, amount: 1 }, { itemId: 60050, amount: 3 }] },
  { id: 20204, postId: 202, level: 4, name: '仙人掌花', weeklyLimit: 8, boxCapacity: 7, weight: 25, materials: [{ itemId: 51033, amount: 1 }, { itemId: 64027, amount: 2 }, { itemId: 60021, amount: 4 }] },
  { id: 20205, postId: 202, level: 5, name: '巨大獠牙化石', weeklyLimit: 3, boxCapacity: 5, weight: 30, materials: [{ itemId: 77420, amount: 1 }, { itemId: 64309, amount: 3 }, { itemId: 67208, amount: 5 }] },
  { id: 20301, postId: 203, level: 1, name: '麦饭石鸡蛋', weeklyLimit: 25, boxCapacity: 10, weight: 15, materials: [{ itemId: 51171, amount: 1 }, { itemId: 63220, amount: 2 }] },
  { id: 20302, postId: 203, level: 2, name: '凯利达鲑鱼', weeklyLimit: 15, boxCapacity: 7, weight: 15, materials: [{ itemId: 67108, amount: 1 }, { itemId: 67205, amount: 1 }] },
  { id: 20303, postId: 203, level: 3, name: '温泉浴剂', weeklyLimit: 10, boxCapacity: 7, weight: 20, materials: [{ itemId: 67206, amount: 3 }, { itemId: 90603, amount: 2 }] },
  { id: 20304, postId: 203, level: 4, name: '大型露营帐篷', weeklyLimit: 8, boxCapacity: 7, weight: 25, materials: [{ itemId: 64305, amount: 1 }, { itemId: 67107, amount: 1 }, { itemId: 60029, amount: 5 }] },
  { id: 20305, postId: 203, level: 5, name: '粉红盐', weeklyLimit: 3, boxCapacity: 5, weight: 30, materials: [{ itemId: 64301, amount: 3 }, { itemId: 45021, amount: 3 }, { itemId: 63221, amount: 3 }] },
  { id: 20401, postId: 204, level: 1, name: '火山泥面膜', weeklyLimit: 25, boxCapacity: 10, weight: 15, materials: [{ itemId: 64026, amount: 2 }, { itemId: 67207, amount: 3 }] },
  { id: 20402, postId: 204, level: 2, name: '熔岩石', weeklyLimit: 15, boxCapacity: 7, weight: 15, materials: [{ itemId: 64029, amount: 2 }, { itemId: 50023, amount: 3 }] },
  { id: 20403, postId: 204, level: 3, name: '伊克西翁的角', weeklyLimit: 10, boxCapacity: 7, weight: 20, materials: [{ itemId: 51205, amount: 3 }, { itemId: 64028, amount: 5 }] },
  { id: 20404, postId: 204, level: 4, name: '火山蜥蜴的蛋', weeklyLimit: 8, boxCapacity: 7, weight: 25, materials: [{ itemId: 64051, amount: 5 }, { itemId: 60022, amount: 5 }, { itemId: 51170, amount: 2 }] },
  { id: 20405, postId: 204, level: 5, name: '拉斯帕黑豹皮', weeklyLimit: 3, boxCapacity: 5, weight: 30, materials: [{ itemId: 67213, amount: 1 }, { itemId: 64155, amount: 2 }, { itemId: 63348, amount: 3 }] },
]
