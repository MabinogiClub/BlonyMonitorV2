<script setup lang="ts">
/**
 * 状态栏组件
 * 放置频道显示、角色名、地区名称、连接状态等信息
 */

import { computed } from 'vue'
import { useAppStore } from '../stores/app'

// 获取应用状态
const appStore = useAppStore()

/**
 * 连接状态样式类
 */
const statusClass = computed(() => {
  return appStore.isConnected ? 'status-connected' : 'status-disconnected'
})

/**
 * 连接状态文本
 */
const statusText = computed(() => {
  return appStore.isConnected ? '已连接' : '未连接'
})

/**
 * 地图显示文本
 */
const mapDisplayText = computed(() => {
  const map = appStore.currentMap
  if (!map) return ''
  // 格式: 城 - 区
  if (map.localName && map.mapName) {
    return `${map.localName} - ${map.mapName}`
  }
  if (map.localName) return map.localName
  if (map.mapName) return map.mapName
  return ''
})

/**
 * 地下城显示文本
 */
const dungeonDisplayText = computed(() => {
  const dungeon = appStore.currentDungeon
  if (!dungeon) return ''

  const parts = [dungeon.dungeonName || `Dungeon ${dungeon.dungeonId}`]
  if (dungeon.difficulty) parts.push(`D${dungeon.difficulty}`)
  if (dungeon.floorCount) parts.push(`${dungeon.floorCount}F`)
  if (dungeon.seed) parts.push(`seed ${dungeon.seed}`)
  return parts.join(' · ')
})

/**
 * 频道显示文本
 * 如果是自动模式并且没识别到，显示 [频道识别中...]
 */
const channelDisplayText = computed(() => {
  if (appStore.autoDetect) {
    // 自动模式
    if (appStore.channelName) {
      return appStore.channelName
    } else {
      return '[频道识别中...]'
    }
  } else {
    // 手动模式
    if (appStore.channelName) {
      return appStore.channelName
    } else {
      // 手动模式但没选择频道，显示选择的频道ID或默认文本
      if (appStore.currentChannelId > 0) {
        // 尝试从配置中查找频道名称
        const config = appStore.channelsConfig
        if (config) {
          const servers = config.servers || config.Servers || []
          for (const server of servers) {
            const channels = server.channels || server.Channels || []
            for (const ch of channels) {
              const chId = ch.id ?? ch.ID
              if (chId === appStore.currentChannelId) {
                const serverName = server.name || server.Name
                const chName = ch.name || ch.Name
                return `${serverName} ${chName}`
              }
            }
          }
        }
        return `频道 ${appStore.currentChannelId}`
      }
      return '未选择频道'
    }
  }
})
</script>

<template>
  <div class="status-bar">
    <!-- 频道名称 -->
    <span class="status-item channel-name">{{ channelDisplayText }}</span>

    <!-- 玩家名称 -->
    <span v-if="appStore.selfInfo" class="status-item self-name">{{ appStore.selfInfo.name }}</span>

    <!-- 地图信息 -->
    <span v-if="mapDisplayText" class="status-item map-name">{{ mapDisplayText }}</span>

    <!-- 地下城信息 -->
    <span v-if="dungeonDisplayText" class="status-item dungeon-name">{{ dungeonDisplayText }}</span>

    <!-- 连接状态 -->
    <span :class="['status-item', 'status', statusClass]">{{ statusText }}</span>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 状态栏样式
 */

.status-bar {
  height: 24px;
  min-height: 24px;
  flex-shrink: 0;
  background: rgba(25, 25, 25, 0.9);
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  font-size: 11px;
  flex: none !important;
}

.status-item {
  color: #aaa;
  font-weight: 500;
}

// 频道名
.channel-name {
  color: #ffc107;
  font-weight: 600;
}

.map-name {
  color: #4fc3f7;
  font-weight: 500;
}

.dungeon-name {
  color: #ce93d8;
  font-weight: 500;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.self-name {
  color: #4caf50;
  font-weight: 600;
}

.status {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 3px;
  margin-left: auto;
}

// 连接状态样式
.status-connected {
  background: rgba(76, 175, 80, 0.3);
  color: #81c784;
}

.status-disconnected {
  background: rgba(244, 67, 54, 0.3);
  color: #e57373;
}
</style>
