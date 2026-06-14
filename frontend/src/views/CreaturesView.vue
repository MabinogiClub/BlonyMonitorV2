<script setup lang="ts">
/**
 * 生物库视图组件
 * 显示所有生物信息
 */

import { ref, onMounted, onUnmounted } from 'vue'
import * as api from '../composables/useApi'

// 生物列表数据
const creatures = ref<CreatureInfo[]>([])

// 定时器 ID
let updateInterval: number | null = null

/**
 * 获取生物条目样式类
 */
function getItemClass(creature: CreatureInfo): string {
  return creature.isAlive ? '' : 'dead'
}

/**
 * 获取状态指示器样式类
 */
function getStatusClass(creature: CreatureInfo): string {
  return creature.isAlive ? 'alive' : 'dead'
}

/**
 * 获取名称样式类
 */
function getNameClass(creature: CreatureInfo): string {
  return creature.isPC ? 'pc' : ''
}

/**
 * 获取类型标签
 */
function getTypeLabel(creature: CreatureInfo): string {
  if (creature.isPC) return '玩家'
  return creature.raceName || `种族${creature.raceId}`
}

/**
 * 更新视图数据
 */
async function updateView() {
  try {
    const newCreatures = await api.getAllCreatures()

    // 检查数据是否有变化，避免不必要的重渲染
    // 但如果本地数据为空，始终更新
    if (creatures.value.length > 0) {
      const currentJson = JSON.stringify(newCreatures)
      const lastJson = JSON.stringify(creatures.value)
      if (currentJson === lastJson) {
        return
      }
    }

    creatures.value = newCreatures
  } catch (e) {
    console.error('Failed to get creatures:', e)
  }
}

onMounted(() => {
  updateView()
  updateInterval = window.setInterval(updateView, 1000)
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
})
</script>

<template>
  <div class="creatures-view">
    <!-- 空状态 -->
    <van-empty
      v-if="!creatures || creatures.length === 0"
      image="search"
      description="等待生物出现..."
    />
    
    <!-- 生物列表 -->
    <div 
      v-else
      v-for="creature in creatures" 
      :key="creature.id"
      class="creature-item"
      :class="getItemClass(creature)"
    >
      <!-- 状态指示器 -->
      <div 
        class="creature-status" 
        :class="getStatusClass(creature)"
        :title="creature.isAlive ? '存活' : '已死亡'"
      ></div>
      
      <!-- 生物信息 -->
      <div class="creature-info">
        <div class="creature-name" :class="getNameClass(creature)">
          {{ creature.name }}
        </div>
        <div class="creature-details">
          <span>{{ getTypeLabel(creature) }}</span>
          <span class="creature-id">{{ creature.id }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 生物库样式
 * 定义生物库的样式
 */

.creatures-view {
  height: 100%;
}

// 生物条目
.creature-item {
  background: rgba(40, 40, 40, 0.6);
  border-radius: 4px;
  margin-bottom: 3px;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 8px;

  // 死亡状态的生物
  &.dead {
    opacity: 0.5;
  }
}

// 生物状态指示器
.creature-status {
  width: 8px;
  height: 8px;
  border-radius: 50%;

  // 存活状态
  &.alive {
    background: #4caf50;
    box-shadow: 0 0 4px #4caf50;
  }

  // 死亡状态
  &.dead {
    background: #666;
  }
}

// 生物信息
.creature-info {
  flex: 1;
  min-width: 0;
}

// 生物名称
.creature-name {
  font-weight: 500;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;

  // 玩家角色名称
  &.pc {
    color: #42a5f5;
  }
}

// 生物详情
.creature-details {
  font-size: 10px;
  color: #888;
  display: flex;
  gap: 8px;
}

// 生物 ID
.creature-id {
  font-family: monospace;
  font-size: 9px;
  color: #666;
}
</style>
