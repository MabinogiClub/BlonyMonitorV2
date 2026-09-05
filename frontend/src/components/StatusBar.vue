<script setup lang="ts">
import { computed } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import { mdiAlert } from '@mdi/js'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

const selfLoadingHint =
  '尚未识别到您的角色，战斗、Buff 等数据可能不准确。请切换地图（进入农场）或进入副本以完成识别。'

const showSelfLoading = computed(() => appStore.isConnected && !appStore.selfInfo)

const statusClass = computed(() => {
  return appStore.isConnected ? 'status-connected' : 'status-disconnected'
})

const statusText = computed(() => {
  return appStore.isConnected ? '已连接' : '未连接'
})

const mapDisplayText = computed(() => {
  const map = appStore.currentMap
  if (!map) return ''

  if ((map.localName === '地下城' || map.localName === '副本') && map.mapName) {
    return map.mapName
  }
  if (map.localName && map.mapName && map.localName === map.mapName) {
    return map.mapName
  }
  if (map.localName && map.mapName) {
    return `${map.localName} - ${map.mapName}`
  }
  if (map.localName) return map.localName
  if (map.mapName) return map.mapName
  return ''
})

const channelDisplayText = computed(() => {
  if (appStore.autoDetect) {
    return appStore.channelName || '[频道识别中...]'
  }

  if (appStore.channelName) {
    return appStore.channelName
  }

  if (appStore.currentChannelId > 0) {
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
})

const selfNameDisplayText = computed(() => {
  const name = appStore.selfInfo?.name || ''
  if (!appStore.maskSelfName || !name) return name

  const characters = Array.from(name)
  if (characters.length <= 1) return name

  const visibleCount = characters.length === 2 ? 1 : 2
  return `${characters.slice(0, visibleCount).join('')}**`
})
</script>

<template>
  <div class="status-bar">
    <span class="status-item channel-name">{{ channelDisplayText }}</span>

    <span v-if="appStore.selfInfo" class="status-item self-name">{{ selfNameDisplayText }}</span>
    <span v-else-if="showSelfLoading" class="status-item self-loading">
      <svg-icon type="mdi" :path="mdiAlert" :size="12" class="self-loading-icon" />
      <span>角色加载中</span>
      <div class="self-loading-popover" role="tooltip">{{ selfLoadingHint }}</div>
    </span>

    <span v-if="mapDisplayText" class="status-item map-name">{{ mapDisplayText }}</span>
    <span :class="['status-item', 'status', statusClass]">{{ statusText }}</span>
  </div>
</template>

<style lang="scss" scoped>
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

.self-loading {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #ff9800;
  font-weight: 600;
  cursor: help;
  z-index: 20;

  &:hover {
    z-index: 100000;
  }
}

.self-loading-icon {
  flex-shrink: 0;
  color: #ff9800;
}

.self-loading-popover {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  width: max-content;
  max-width: 280px;
  padding: 8px 10px;
  background: rgba(50, 50, 50, 0.98);
  color: #eee;
  font-size: 11px;
  font-weight: 400;
  line-height: 1.5;
  white-space: normal;
  border: 1px solid rgba(255, 152, 0, 0.35);
  border-radius: 6px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.45);
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transition: opacity 0.2s, visibility 0.2s;
  z-index: 100000;

  &::before {
    content: '';
    position: absolute;
    bottom: 100%;
    left: 10px;
    border: 5px solid transparent;
    border-bottom-color: rgba(50, 50, 50, 0.98);
  }

  &::after {
    content: '';
    position: absolute;
    bottom: 100%;
    left: 9px;
    border: 6px solid transparent;
    border-bottom-color: rgba(255, 152, 0, 0.35);
    z-index: -1;
  }
}

.self-loading:hover .self-loading-popover {
  opacity: 1;
  visibility: visible;
}

.status {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 3px;
  margin-left: auto;
}

.status-connected {
  background: rgba(76, 175, 80, 0.3);
  color: #81c784;
}

.status-disconnected {
  background: rgba(244, 67, 54, 0.3);
  color: #e57373;
}
</style>
