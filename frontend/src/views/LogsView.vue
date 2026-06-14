<script setup lang="ts">
/**
 * 事件日志视图组件
 * 显示战斗事件日志，支持过滤
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { formatNumber, formatTime, getConditionName, getConditionColorClass } from '../composables/useUtils'

// 获取应用状态
const appStore = useAppStore()

// 事件日志数据
const logs = ref<EventLog[]>([])

// 定时器 ID
let updateInterval: number | null = null

/**
 * 过滤后的日志
 */
const filteredLogs = computed(() => {
  return logs.value.filter(log => appStore.logFilters[log.type as keyof typeof appStore.logFilters])
})

/**
 * 切换过滤器
 */
function toggleFilter(type: 'damage' | 'appear' | 'condition' | 'finish') {
  appStore.toggleLogFilter(type)
}

/**
 * 获取状态名称
 */
function getConditionDisplayName(log: EventLog): string {
  return log.conditionName || getConditionName(log.conditionId || 0)
}

/**
 * 将颜色类转换为 Vant Tag 类型
 */
function getTagType(colorClass: string): 'primary' | 'success' | 'warning' | 'danger' | 'default' {
  switch (colorClass) {
    case 'condition-attack':
      return 'danger'
    case 'condition-magic':
      return 'primary'
    case 'condition-song':
      return 'warning'
    case 'condition-pierce':
      return 'success'
    default:
      return 'default'
  }
}

/**
 * 更新视图数据
 */
async function updateView() {
  try {
    logs.value = await api.getEventLogs(100, 'all')
  } catch (e) {
    console.error('Failed to get event logs:', e)
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
  <div class="logs-view">
    <!-- 过滤器 -->
    <div class="log-filters">
      <van-tag
        :type="appStore.logFilters.damage ? 'danger' : 'default'"
        :plain="!appStore.logFilters.damage"
        size="medium"
        @click="toggleFilter('damage')"
        class="filter-tag"
      >
        伤害
      </van-tag>
      <van-tag
        :type="appStore.logFilters.appear ? 'success' : 'default'"
        :plain="!appStore.logFilters.appear"
        size="medium"
        @click="toggleFilter('appear')"
        class="filter-tag"
      >
        出现
      </van-tag>
      <van-tag
        :type="appStore.logFilters.condition ? 'primary' : 'default'"
        :plain="!appStore.logFilters.condition"
        size="medium"
        @click="toggleFilter('condition')"
        class="filter-tag"
      >
        状态
      </van-tag>
      <van-tag
        :type="appStore.logFilters.finish ? 'warning' : 'default'"
        :plain="!appStore.logFilters.finish"
        size="medium"
        @click="toggleFilter('finish')"
        class="filter-tag"
      >
        击杀
      </van-tag>
    </div>

    <!-- 日志列表容器 -->
    <div class="log-list-wrapper">
      <!-- 空状态 -->
      <van-empty
        v-if="!logs || logs.length === 0"
        image="search"
        description="等待事件日志..."
      />

      <!-- 日志列表 -->
      <div v-else class="log-list-container">
      <!-- 伤害日志 -->
      <template v-for="log in filteredLogs" :key="`${log.type}-${log.at}`">
        <!-- 伤害事件 -->
        <div
          v-if="log.type === 'damage'"
          class="log-item"
          style="background: rgba(244, 67, 54, 0.1);"
        >
          <span class="log-time">{{ formatTime(log.at) }}</span>
          <span class="log-type log-type-damage">伤害</span>
          <span class="log-message">
            <template v-if="!log.isPC && log.raceName">[{{ log.raceName }}]</template>
            {{ log.entityName }} →
            <template v-if="!log.targetIsPC && log.targetRaceName">[{{ log.targetRaceName }}]</template>
            {{ log.targetName }}
            [{{ log.skillName }}]
            <span class="log-damage-value">{{ formatNumber(log.damage || 0) }}</span>
            <span v-if="log.isCritical" class="log-crit">暴击</span>
          </span>
        </div>

        <!-- 出现事件 -->
        <div
          v-else-if="log.type === 'appear'"
          class="log-item"
          style="background: rgba(76, 175, 80, 0.1);"
        >
          <span class="log-time">{{ formatTime(log.at) }}</span>
          <span class="log-type log-type-appear">出现</span>
          <span class="log-message">
            <template v-if="!log.isPC && log.raceName">[{{ log.raceName }}]</template>
            {{ log.entityName }}
            {{ log.isPC ? '(玩家)' : '' }}
          </span>
        </div>

        <!-- 状态事件 -->
        <div
          v-else-if="log.type === 'condition'"
          class="log-item"
          style="background: rgba(156, 39, 176, 0.1);"
        >
          <span class="log-time">{{ formatTime(log.at) }}</span>
          <span class="log-type log-type-condition">{{ log.isEnable ? '状态+' : '状态-' }}</span>
          <span class="log-message" style="display: flex; align-items: center; gap: 4px;">
            <template v-if="!log.isPC && log.raceName">[{{ log.raceName }}]</template>
            {{ log.entityName }}
            <van-tag
              :type="getTagType(getConditionColorClass(getConditionDisplayName(log)))"
              size="medium"
              plain
              style="margin: 0 4px;"
            >
              {{ getConditionDisplayName(log) }}
            </van-tag>
            <template v-if="log.attackerName">← {{ log.attackerName }}</template>
          </span>
        </div>

        <!-- 击杀事件 -->
        <div
          v-else-if="log.type === 'finish'"
          class="log-item"
          style="background: rgba(255, 152, 0, 0.1);"
        >
          <span class="log-time">{{ formatTime(log.at) }}</span>
          <span class="log-type log-type-finish">击杀</span>
          <span class="log-message">
            <template v-if="!log.isPC && log.raceName">[{{ log.raceName }}]</template>
            {{ log.entityName }} 被击败
            <template v-if="log.attackerName">← {{ log.attackerName }}</template>
          </span>
        </div>
      </template>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 日志样式
 * 定义事件日志列表的样式
 */

.logs-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

// 日志过滤器 - 固定在顶部
.log-filters {
  display: flex;
  gap: 8px;
  padding: 8px;
  background: rgba(30, 30, 30, 0.95);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  flex-wrap: wrap;
  flex-shrink: 0;
}

// 过滤器 tag 样式
.filter-tag {
  cursor: pointer;
  user-select: none;
  transition: all 0.2s;

  &:hover {
    opacity: 0.8;
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(0);
  }
}

// 日志列表包装器 - 可滚动区域
.log-list-wrapper {
  flex: 1;
  overflow-y: auto;
  min-height: 0;

  // 滚动条样式
  &::-webkit-scrollbar {
    width: 4px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 2px;
  }
}

// 日志列表容器
.log-list-container {
  padding: 6px;
}

// 日志项
.log-item {
  padding: 4px 8px;
  border-radius: 3px;
  margin-bottom: 2px;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 6px;
}

// 日志时间
.log-time {
  color: #888;
  font-size: 10px;
  min-width: 50px;
}

// 日志类型标签
.log-type {
  padding: 1px 4px;
  border-radius: 2px;
  font-size: 9px;
  min-width: 40px;
  text-align: center;
}

// 伤害类型
.log-type-damage {
  background: rgba(244, 67, 54, 0.3);
  color: #ef5350;
}

// 出现类型
.log-type-appear {
  background: rgba(76, 175, 80, 0.3);
  color: #81c784;
}

// 状态类型
.log-type-condition {
  background: rgba(156, 39, 176, 0.3);
  color: #ba68c8;
}

// 击杀类型
.log-type-finish {
  background: rgba(255, 152, 0, 0.3);
  color: #ffb74d;
}

// 日志消息
.log-message {
  flex: 1;
  color: #ddd;
}

// 伤害值
.log-damage-value {
  color: #ffc107;
  font-weight: 500;
}

// 暴击标记
.log-crit {
  color: #ff5722;
}

// Vant Tag 样式覆盖
:deep(.van-tag) {
  --van-tag-font-size: 9px;
  --van-tag-padding: 2px 6px;
  --van-tag-border-radius: 3px;
}
</style>
