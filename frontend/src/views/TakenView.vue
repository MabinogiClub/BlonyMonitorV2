<script setup lang="ts">
/**
 * 受到伤害视图组件
 * 显示受到的伤害统计，支持查看详细伤害来源
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { formatNumber, formatDuration, getDisplayName, getSkillName, getSkillIconUrl, BAR_CLASSES, throttle } from '../composables/useUtils'

// 获取应用状态
const appStore = useAppStore()

// 受到伤害统计数据
const stats = ref<TakenStats[]>([])

let updateInterval: number | null = null

/**
 * 当前选中的目标
 */
const selectedTarget = computed(() => appStore.selectedTarget)

/**
 * 当前目标的详细数据
 */
const targetDetail = computed(() => {
  if (!selectedTarget.value) return null
  return stats.value.find(t => t.id === selectedTarget.value!.id)
})

/**
 * 获取进度条样式类
 */
function getBarClass(index: number): string {
  return BAR_CLASSES[index % BAR_CLASSES.length]
}

/**
 * 选择目标查看详情
 */
function selectTarget(targetId: string, targetName: string) {
  appStore.setSelectedTarget({ id: targetId, name: targetName })
}

/**
 * 返回目标列表
 */
function goBack() {
  appStore.clearSelectedTarget()
}

/**
 * 切换展开状态
 */
function toggleExpand(attackerId: string) {
  appStore.toggleExpanded('taken-' + attackerId)
}

/**
 * 检查是否展开
 */
function isExpanded(attackerId: string): boolean {
  return appStore.isExpanded('taken-' + attackerId)
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
    stats.value = await api.getDamageTaken()
  } catch (e) {
    console.error('Failed to get taken stats:', e)
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
  <div class="taken-view">
    <!-- 空状态 -->
    <van-empty
      v-if="!stats || stats.length === 0"
      image="search"
      description="等待战斗数据..."
    />
    
    <!-- 目标详情视图 -->
    <template v-else-if="selectedTarget && targetDetail">
      <!-- 返回按钮 -->
      <div
        class="damage-item back-btn"
        style="background: rgba(66, 165, 245, 0.2); cursor: pointer;"
        @click="goBack"
      >
        <div class="damage-content">
          <span class="damage-name">← 返回列表</span>
          <div class="damage-info">
            <span class="damage-duration">{{ formatDuration(targetDetail.duration) }}</span>
            <span class="damage-dps hover-tip" :data-tooltip="targetDetail.dps.toLocaleString() + '/s'">
              {{ formatNumber(targetDetail.dps) }}/s
            </span>
            <span class="damage-value">{{ getDisplayName(targetDetail.id, targetDetail.name) }}</span>
            <span class="damage-percent hover-tip" :data-tooltip="'总计 ' + targetDetail.totalDamage.toLocaleString()">
              总计 {{ formatNumber(targetDetail.totalDamage) }}
            </span>
          </div>
        </div>
      </div>
      
      <!-- 提示信息 -->
      <div style="margin: 8px 0; padding: 6px 8px; background: rgba(40, 40, 40, 0.8); border-radius: 4px; font-size: 11px; color: #aaa;">
        对 {{ getDisplayName(targetDetail.id, targetDetail.name) }} 造成伤害的所有来源：
      </div>
      
      <!-- 攻击者列表 -->
      <template v-for="(attacker, index) in targetDetail.attackers" :key="attacker.id">
        <div
          class="damage-item"
          :class="{
            expandable: attacker.skills && attacker.skills.length > 0,
            expanded: isExpanded(attacker.id)
          }"
          @click="attacker.skills && attacker.skills.length > 0 && toggleExpand(attacker.id)"
        >
          <div
            class="damage-bar"
            :class="getBarClass(index)"
            :style="{ width: `${(attacker.totalDamage / (targetDetail.attackers[0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
          ></div>
          <div class="damage-content">
            <span class="damage-name">
              <span class="damage-name-wrapper">
                <span v-if="attacker.skills && attacker.skills.length > 0" class="expand-icon">▶</span>
                <span class="damage-name-text">{{ getDisplayName(attacker.id, attacker.name) }}</span>
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
        <div 
          v-if="attacker.skills && attacker.skills.length > 0" 
          class="sub-items"
          v-show="isExpanded(attacker.id)"
        >
          <div 
            v-for="(skill, skillIndex) in attacker.skills" 
            :key="skill.skillId"
            class="sub-item"
          >
            <div 
              class="sub-item-bar" 
              :class="getBarClass((index + skillIndex) % BAR_CLASSES.length)"
              :style="{ width: `${(skill.totalDamage / (attacker.skills![0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
            ></div>
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
    
    <!-- 目标列表视图 -->
    <template v-else>
      <div
        v-for="(target, index) in stats"
        :key="target.id"
        class="damage-item expandable target-item"
        @click="selectTarget(target.id, target.name)"
      >
        <div
          class="damage-bar"
          :class="getBarClass(index)"
          :style="{ width: `${(target.totalDamage / (stats[0]?.totalDamage || 1) * 100).toFixed(1)}%` }"
        ></div>
        <div class="damage-content">
          <span class="damage-name">
            <span class="damage-name-wrapper">
              <span class="expand-icon">▶</span>
              <span class="damage-name-text">{{ getDisplayName(target.id, target.name) }}</span>
            </span>
            <span v-if="target.status" class="status-badge" :class="getStatusClass(target.status)">
              {{ getStatusText(target.status) }}
            </span>
          </span>
          <div class="damage-info">
            <span class="damage-duration">{{ formatDuration(target.duration) }}</span>
            <span class="damage-dps hover-tip" :data-tooltip="target.dps.toLocaleString() + '/s'">
              {{ formatNumber(target.dps) }}/s
            </span>
            <span class="damage-hits">{{ target.attackers.length }}个来源</span>
            <span class="damage-value hover-tip" :data-tooltip="target.totalDamage.toLocaleString()">
              {{ formatNumber(target.totalDamage) }}
            </span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.taken-view {
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

.damage-duration {
  color: #8bc34a;
  font-size: 11px;
}

.damage-dps {
  color: #ff9800;
  font-size: 11px;
}

/* 带水印的数值容器 */
</style>
