<script setup lang="ts">
/**
 * 造成伤害视图组件
 * 显示按技能分组的伤害统计
 */

import { ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { formatNumber, getSkillName, getSkillIconUrl, BAR_CLASSES, throttle } from '../composables/useUtils'

// 获取应用状态
const appStore = useAppStore()

// 伤害统计数据
const stats = ref<AttackerStats[]>([])

let updateInterval: number | null = null

/**
 * 获取最大伤害值
 */
function getMaxDamage(): number {
  return stats.value[0]?.totalDamage || 1
}

/**
 * 计算进度条宽度
 */
function getBarWidth(damage: number): string {
  return `${(damage / getMaxDamage() * 100).toFixed(1)}%`
}

/**
 * 获取进度条样式类
 */
function getBarClass(index: number): string {
  return BAR_CLASSES[index % BAR_CLASSES.length]
}

/**
 * 切换展开状态
 */
function toggleExpand(attackerId: string) {
  appStore.toggleExpanded('skill-' + attackerId)
}

/**
 * 检查是否展开
 */
function isExpanded(attackerId: string): boolean {
  return appStore.isExpanded('skill-' + attackerId)
}

/**
 * 格式化状态文本
 */
function getStatusText(status: string): string {
  switch (status) {
    case 'active':
      return '战斗中'
    case 'idle':
      return '空闲'
    case 'dead':
      return '死亡'
    default:
      return ''
  }
}

/**
 * 获取状态样式类
 */
function getStatusClass(status: string): string {
  switch (status) {
    case 'active':
      return 'status-active'
    case 'idle':
      return 'status-idle'
    case 'dead':
      return 'status-dead'
    default:
      return ''
  }
}

/**
 * 更新视图数据
 */
async function updateView() {
  try {
    stats.value = await api.getDamageBySkill()
  } catch (e) {
    console.error('Failed to get skill stats:', e)
  }
}

// 节流后的更新函数（最小间隔200ms）
const throttledUpdateView = throttle(updateView, 200)

onMounted(async () => {
  updateView()

  // 监听后端的 DPS 更新事件（使用节流）
  api.onEvent('dps-update', throttledUpdateView)

  // 监听清理事件
  api.onEvent('clear', () => {
    stats.value = []
  })
})

onUnmounted(() => {
  if (updateInterval) {
    clearInterval(updateInterval)
  }
  // 取消事件监听
  api.offEvent('dps-update')
  api.offEvent('clear')
})
</script>

<template>
  <div class="damage-view">
    <!-- 空状态 -->
    <van-empty
      v-if="!stats || stats.length === 0"
      image="search"
      description="等待战斗数据..."
    />
    
    <!-- 伤害列表 -->
    <template v-else>
      <template v-for="(attacker, index) in stats" :key="attacker.id">
        <!-- 攻击者条目 -->
        <div 
          class="damage-item expandable"
          :class="{ expanded: isExpanded(attacker.id) }"
          @click="toggleExpand(attacker.id)"
        >
          <!-- 进度条背景 -->
          <div 
            class="damage-bar" 
            :class="getBarClass(index)"
            :style="{ width: getBarWidth(attacker.totalDamage) }"
          ></div>
          
          <!-- 内容 -->
          <div class="damage-content">
            <span class="damage-name">
              <span class="damage-name-wrapper">
                <span class="expand-icon">▶</span>
                <span class="damage-name-text">{{ attacker.name }}</span>
              </span>
              <span v-if="attacker.status" class="status-badge" :class="getStatusClass(attacker.status)">
                {{ getStatusText(attacker.status) }}
              </span>
            </span>
            <div class="damage-info">
              <span class="damage-dps hover-tip" :data-tooltip="attacker.dps.toLocaleString() + '/s'">
                {{ formatNumber(attacker.dps) }}/s
              </span>
              <span class="damage-value hover-tip" :data-tooltip="attacker.totalDamage.toLocaleString()">
                {{ formatNumber(attacker.totalDamage) }}
              </span>
              <span class="damage-percent">{{ attacker.percent.toFixed(1) }}%</span>
            </div>
          </div>
        </div>
        
        <!-- 技能子条目 -->
        <div class="sub-items" v-show="isExpanded(attacker.id)">
          <div 
            v-for="(skill, skillIndex) in attacker.skills" 
            :key="skill.skillId"
            class="sub-item"
          >
            <!-- 子条目进度条 -->
            <div 
              class="sub-item-bar" 
              :class="getBarClass((index + skillIndex) % BAR_CLASSES.length)"
              :style="{ width: `${(skill.totalDamage / (attacker.skills[0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
            ></div>
            
            <!-- 子条目内容 -->
            <div class="sub-item-content">
              <div class="sub-item-name">
                <img 
                  :src="getSkillIconUrl(skill.skillId)" 
                  alt=""
                  width="20" 
                  height="20" 
                  style="border-radius: 2px;"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                >
                <span>{{ getSkillName(skill.skillId) }}</span>
              </div>
              <div class="sub-item-stats">
                <span>{{ skill.hitCount }}次</span>
                <span>{{ skill.critCount }}暴击</span>
                <span class="hover-tip" :data-tooltip="skill.avgDamage.toLocaleString()">
                  平均{{ formatNumber(skill.avgDamage) }}
                </span>
                <span class="hover-tip" :data-tooltip="`${skill.minDamage.toLocaleString()} ~ ${skill.maxDamage.toLocaleString()}`">{{ formatNumber(skill.minDamage) }}~{{ formatNumber(skill.maxDamage) }}</span>
                <span class="hover-tip sub-item-damage" :data-tooltip="skill.totalDamage.toLocaleString()">
                  {{ formatNumber(skill.totalDamage) }}
                </span>
                <span>{{ skill.percent.toFixed(1) }}%</span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.damage-view {
  height: 100%;
}

.sub-item-name {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* 状态标签样式 */
.status-badge {
  display: inline-block;
  margin-left: 6px;
  padding: 2px 6px;
  font-size: 10px;
  border-radius: 3px;
  font-weight: 500;
}

.status-active {
  background: rgba(76, 175, 80, 0.3);
  color: #4caf50;
  border: 1px solid rgba(76, 175, 80, 0.5);
}

.status-idle {
  background: rgba(158, 158, 158, 0.3);
  color: #9e9e9e;
  border: 1px solid rgba(158, 158, 158, 0.5);
}

.status-dead {
  background: rgba(244, 67, 54, 0.3);
  color: #f44336;
  border: 1px solid rgba(244, 67, 54, 0.5);
}
</style>
