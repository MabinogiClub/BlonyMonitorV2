<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import SvgIcon from '@jamescoyle/vue-icon'
import {
  mdiAlertCircleOutline,
  mdiCheckCircleOutline,
  mdiClockOutline,
  mdiDownload,
  mdiMessageAlertOutline,
  mdiOpenInNew,
  mdiRefresh,
} from '@mdi/js'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import * as api from '../composables/useApi'

const LATEST_RELEASE_URL = 'https://github.com/MabinogiClub/BlonyMonitorV2/releases/latest'
const FEEDBACK_URL = 'https://github.com/MabinogiClub/BlonyMonitorV2/issues'
const RELEASE_API_URL = 'https://api.github.com/repos/MabinogiClub/BlonyMonitorV2/releases/latest'

type UpdateState = 'idle' | 'checking' | 'latest' | 'available' | 'error'

const currentVersion = ref('dev')
const latestVersion = ref('')
const updateState = ref<UpdateState>('idle')

const clockNow = ref(Date.now())
let clockTimer: number | undefined

// Calibrated against the in-game 10-minute display during three logins.
const EIRIN_PHASE_OFFSET_MINUTES = -14

const realTimeLabel = computed(() => {
  const now = new Date(clockNow.value)
  return [now.getHours(), now.getMinutes(), now.getSeconds()]
    .map(value => String(value).padStart(2, '0'))
    .join(':')
})

const mabiTimeLabel = computed(() => {
  // The game clock runs 40 times faster than real time: one Eirin day is 36
  // real minutes. The +08:00 anchor matches the conversion used by ruawork.js.
  const cycleMinutes = Math.floor(((clockNow.value + 8 * 60 * 60 * 1000) % (36 * 60 * 1000)) / 1500)
  const gameMinutes = (cycleMinutes + EIRIN_PHASE_OFFSET_MINUTES + 1440) % 1440
  const hour = Math.floor(gameMinutes / 60) % 24
  const minute = gameMinutes % 60
  return String(hour).padStart(2, '0') + ':' + String(minute).padStart(2, '0')
})

const currentVersionLabel = computed(() => {
  if (currentVersion.value === 'dev') return '开发版'
  return currentVersion.value.startsWith('v') ? currentVersion.value : `v${currentVersion.value}`
})

const latestVersionLabel = computed(() => {
  if (!latestVersion.value) return ''
  return latestVersion.value.startsWith('v') ? latestVersion.value : `v${latestVersion.value}`
})

const updateMessage = computed(() => {
  switch (updateState.value) {
    case 'checking':
      return '正在检查更新...'
    case 'latest':
      return latestVersionLabel.value ? `已是最新版 ${latestVersionLabel.value}` : '已是最新版'
    case 'available':
      return latestVersionLabel.value ? `发现新版本 ${latestVersionLabel.value}` : '发现新版本'
    case 'error':
      return '检查失败，请重试'
    default:
      return '尚未检查更新'
  }
})

const updateIcon = computed(() => {
  if (updateState.value === 'error') return mdiAlertCircleOutline
  if (updateState.value === 'latest') return mdiCheckCircleOutline
  return mdiRefresh
})

function parseVersion(version: string): [number, number, number] | null {
  const match = version.trim().replace(/^v/i, '').match(/^(\d+)\.(\d+)\.(\d+)/)
  if (!match) return null
  return [Number(match[1]), Number(match[2]), Number(match[3])]
}

function compareVersions(left: string, right: string): number {
  const leftParts = parseVersion(left)
  const rightParts = parseVersion(right)
  if (!leftParts || !rightParts) return 0

  for (let index = 0; index < leftParts.length; index += 1) {
    if (leftParts[index] !== rightParts[index]) {
      return leftParts[index] > rightParts[index] ? 1 : -1
    }
  }
  return 0
}

function openExternal(url: string) {
  if (typeof window !== 'undefined' && window.runtime) {
    BrowserOpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

async function loadCurrentVersion() {
  try {
    currentVersion.value = await api.getClientVersion()
  } catch (error) {
    console.error('加载客户端版本失败:', error)
  }
}

async function checkForUpdates() {
  if (updateState.value === 'checking') return

  updateState.value = 'checking'
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 5000)
  try {
    const response = await fetch(RELEASE_API_URL, {
      headers: { Accept: 'application/vnd.github+json' },
      signal: controller.signal,
    })

    if (!response.ok) throw new Error(`GitHub release request failed: ${response.status}`)
    const release = await response.json() as { tag_name?: string }
    latestVersion.value = release.tag_name || ''

    const isDevelopmentBuild = currentVersion.value === 'dev' || !parseVersion(currentVersion.value)
    updateState.value = isDevelopmentBuild || compareVersions(latestVersion.value, currentVersion.value) > 0
      ? 'available'
      : 'latest'
  } catch (error) {
    updateState.value = 'error'
    console.error('检查 GitHub 更新失败:', error)
  } finally {
    window.clearTimeout(timeout)
  }
}

onMounted(() => {
  clockTimer = window.setInterval(() => {
    clockNow.value = Date.now()
  }, 1000)
  void loadCurrentVersion()
  void checkForUpdates()
})

onBeforeUnmount(() => {
  if (clockTimer !== undefined) window.clearInterval(clockTimer)
})
</script>

<template>
  <div class="home-view">
    <section class="hero" aria-labelledby="home-title">
      <div class="hero-content">
        <span class="hero-eyebrow">BlonyMonitorV2</span>
        <h1 id="home-title">布罗妮大调查</h1>
        <p>记录每一次战斗，洞察每一份数据</p>
      </div>
    </section>

    <section class="welcome-content">
      <p class="intro-copy">
        面向《洛奇》冒险者的战斗数据监控工具，帮助你更清晰地了解伤害、角色状态与战斗过程。
      </p>

      <div class="action-links" aria-label="项目链接">
        <a
          class="action-link primary"
          :href="LATEST_RELEASE_URL"
          target="_blank"
          rel="noreferrer"
          @click.prevent="openExternal(LATEST_RELEASE_URL)"
        >
          <svg-icon type="mdi" :path="mdiDownload" :size="16" />
          <span>下载最新版</span>
          <svg-icon type="mdi" :path="mdiOpenInNew" :size="13" class="external-icon" />
        </a>
        <a
          class="action-link"
          :href="FEEDBACK_URL"
          target="_blank"
          rel="noreferrer"
          @click.prevent="openExternal(FEEDBACK_URL)"
        >
          <svg-icon type="mdi" :path="mdiMessageAlertOutline" :size="16" />
          <span>提交反馈</span>
          <svg-icon type="mdi" :path="mdiOpenInNew" :size="13" class="external-icon" />
        </a>
      </div>

      <section class="clock-panel" aria-label="时间换算">
        <div class="clock-panel-heading">
          <svg-icon type="mdi" :path="mdiClockOutline" :size="15" />
          <span>时间</span>
        </div>
        <div class="clock-values">
          <div class="clock-value">
            <span class="clock-label">爱琳时间</span>
            <strong>{{ mabiTimeLabel }}</strong>
          </div>
          <div class="clock-divider" aria-hidden="true"></div>
          <div class="clock-value">
            <span class="clock-label">现实时间</span>
            <strong>{{ realTimeLabel }}</strong>
          </div>
        </div>
        <p class="clock-note">按 36 分钟 = 爱琳 24 小时换算；服务器显示可能存在少量校时差异。</p>
      </section>

      <div class="version-row">
        <div class="version-info">
          <span class="version-label">当前版本</span>
          <strong>{{ currentVersionLabel }}</strong>
        </div>
        <button
          type="button"
          class="update-check"
          :class="updateState"
          :disabled="updateState === 'checking'"
          :title="updateMessage"
          @click="checkForUpdates"
        >
          <svg-icon type="mdi" :path="updateIcon" :size="14" :spin="updateState === 'checking'" />
          <span>{{ updateMessage }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.home-view {
  min-height: 100%;
  overflow: hidden;
  color: #f2f2f2;
  background: #171717;
}

.hero {
  min-height: 238px;
  position: relative;
  display: flex;
  align-items: flex-end;
  isolation: isolate;
  background-image: url('../assets/home-hero.png');
  background-position: center 28%;
  background-size: cover;

  &::before {
    content: '';
    position: absolute;
    inset: 0;
    z-index: -1;
    background: linear-gradient(180deg, rgba(15, 15, 15, 0.02) 46%, rgba(23, 23, 23, 0.2) 70%, #171717 100%);
  }
}

.hero-content {
  width: 100%;
  padding: 0 18px 25px;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.7);
}

.hero-eyebrow {
  display: block;
  margin-bottom: 7px;
  color: #a9d8f6;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1.5px;
}

h1 {
  font-size: 25px;
  font-weight: 600;
  line-height: 1.15;
}

.hero-content p {
  margin-top: 7px;
  color: rgba(255, 255, 255, 0.82);
  font-size: 12px;
}

.welcome-content {
  padding: 0 18px 18px;
}

.intro-copy {
  max-width: 390px;
  margin: -3px 0 18px;
  color: #aaa;
  font-size: 12px;
  line-height: 1.7;
}

.action-links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.action-link {
  min-width: 0;
  min-height: 38px;
  padding: 0 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 5px;
  color: #d2d2d2;
  background: rgba(255, 255, 255, 0.05);
  font-size: 11px;
  text-decoration: none;
  transition: background-color 0.15s, border-color 0.15s;

  &:hover {
    border-color: rgba(144, 202, 249, 0.55);
    background: rgba(144, 202, 249, 0.12);
    color: #fff;
  }

  &.primary {
    border-color: rgba(66, 165, 245, 0.5);
    color: #d8efff;
    background: rgba(66, 165, 245, 0.2);
  }
}

.external-icon {
  opacity: 0.65;
}

.clock-panel {
  margin-top: 16px;
  padding: 12px 13px 11px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.035);
}

.clock-panel-heading {
  display: flex;
  align-items: center;
  gap: 5px;
  color: #a9d8f6;
  font-size: 11px;
  font-weight: 600;
}

.clock-values {
  display: flex;
  align-items: center;
  margin-top: 9px;
}

.clock-value {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.clock-label {
  color: #888;
  font-size: 10px;
  white-space: nowrap;
}

.clock-value strong {
  color: #f2f2f2;
  font-size: 20px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  letter-spacing: 0;
  white-space: nowrap;
}

.clock-divider {
  width: 1px;
  height: 22px;
  margin: 0 12px;
  background: rgba(255, 255, 255, 0.14);
}

.clock-note {
  margin-top: 8px;
  color: #737373;
  font-size: 10px;
  line-height: 1.45;
}

.version-row {
  min-height: 42px;
  margin-top: 16px;
  padding-top: 11px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.version-info {
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 7px;

  strong {
    overflow: hidden;
    color: #ddd;
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.version-label {
  flex-shrink: 0;
  color: #777;
  font-size: 10px;
}

.update-check {
  min-width: 0;
  padding: 3px 0 3px 6px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 0;
  color: #999;
  background: transparent;
  font-family: inherit;
  font-size: 10px;
  cursor: pointer;

  span {
    overflow: hidden;
    max-width: 180px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &:hover:not(:disabled) { color: #ddd; }
  &:disabled { cursor: wait; }
  &.latest { color: #81c784; }
  &.available { color: #ffd54f; }
  &.error { color: #ef9a9a; }
}

@media (max-width: 390px) {
  .hero-content,
  .welcome-content { padding-right: 12px; padding-left: 12px; }

  .clock-value { align-items: flex-start; flex-direction: column; gap: 2px; }
  .clock-divider { margin-right: 8px; margin-left: 8px; }

  .version-row { align-items: flex-start; flex-direction: column; }
  .update-check { padding-left: 0; }
}
</style>
