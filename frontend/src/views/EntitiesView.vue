<script setup lang="ts">
/**
 * 角色列表视图组件
 * 显示所有 PC 角色及其状态
 */

import { ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '../stores/app'
import * as api from '../composables/useApi'
import { getConditionName, getConditionColorClass } from '../composables/useUtils'

// 获取应用状态
const appStore = useAppStore()

// 实体列表数据
const entities = ref<EntityInfo[]>([])

// 定时器 ID
let updateInterval: number | null = null

/**
 * 获取状态标签
 */
function getConditionTags(entity: EntityInfo): Array<{ name: string; colorClass: string }> {
  if (!entity.conditions || entity.conditions.length === 0) {
    return []
  }
  
  return entity.conditions.map((condId, idx) => {
    const condName = entity.conditionNames?.[idx] || getConditionName(condId)
    return {
      name: condName,
      colorClass: getConditionColorClass(condName)
    }
  })
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

/* 格式化种族 */
function formatRace(raceId: number): string {
  if (raceId === 10001) {
    return '人类 女性'
  }
  if (raceId === 10002) {
    return '人类 男性'
  }
  if (raceId === 9001) {
    return '精灵 女性'
  }
  if (raceId === 9002) {
    return '精灵 男性'
  }
  if (raceId === 8001) {
    return '巨人 女性'
  }
  if (raceId === 8002) {
    return '巨人 男性'
  }
  return '未知'
}


/**
 * 格式化身高显示 (游戏内身高范围大约 -1.0 到 2.0)
 */
function formatHeight(h: number | undefined): string {
  if (h === undefined) return '-'
  return h.toFixed(2)
}

function formatStat(value: number | undefined): string {
  if (value === undefined || value === null) return '-'
  return Math.round(value).toLocaleString()
}

function statPercent(value: number | undefined, max: number | undefined): number {
  if (!value || !max || max <= 0) return 0
  return Math.max(0, Math.min(100, (value / max) * 100))
}

function hasVitals(entity: EntityInfo): boolean {
  return !!(entity.maxHp || entity.maxMp || entity.maxStamina)
}

function hasBodyInfo(entity: EntityInfo): boolean {
  return entity.height !== undefined || entity.weight !== undefined || entity.upper !== undefined || entity.lower !== undefined
}

function hasTitleInfo(entity: EntityInfo): boolean {
  return !!(entity.titleId || entity.subTitleId || entity.styleTitleId || entity.styleSubTitleId)
}

/**
 * 更新视图数据
 */
async function updateView() {
  try {
    const newEntities = await api.getAllPCEntities()

    // 检查数据是否有变化，避免不必要的重渲染
    const currentJson = JSON.stringify(newEntities)
    if (currentJson === appStore.lastEntitiesJson && entities.value.length > 0) {
      // 只有当本地已有数据且数据未变化时才跳过更新
      return
    }
    appStore.lastEntitiesJson = currentJson

    entities.value = newEntities
  } catch (e) {
    console.error('Failed to get entities:', e)
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
  <div class="entities-view">
    <!-- 空状态 -->
    <van-empty
      v-if="!entities || entities.length === 0"
      image="search"
      description="等待角色出现..."
    />

    <!-- 角色列表 -->
    <div
      v-else
      v-for="entity in entities"
      :key="entity.id"
      class="entity-item"
      :class="{ 'self-entity': entity.isSelf }"
    >
      <!-- 角色图标 -->
      <div class="entity-icon">👤</div>

      <!-- 角色信息 -->
      <div class="entity-info">
        <div class="entity-header">
          <div class="entity-name-container">
            <strong v-if="entity.isSelf" class="self-tag">自己</strong>
            <span class="entity-name" :class="{ 'self-name': entity.isSelf }">
              {{ entity.name }}
            </span>
            <strong v-if="entity.guildName" class="entity-guild">&lt;{{entity.guildName}}&gt;</strong>
          </div>
          <span class="entity-cp" v-if="entity.combatPower">⚔️ {{ entity.combatPower }}</span>
        </div>
        <div class="entity-race">Race: {{ entity.raceId }} &nbsp; {{ formatRace(entity.raceId) }}</div>

        <div v-if="hasVitals(entity)" class="entity-vitals">
          <div class="vital-row hp">
            <span>HP</span>
            <div class="vital-bar"><i :style="{ width: statPercent(entity.hp, entity.maxHp) + '%' }"></i></div>
            <b>{{ formatStat(entity.hp) }}/{{ formatStat(entity.maxHp) }}</b>
          </div>
          <div class="vital-row mp">
            <span>MP</span>
            <div class="vital-bar"><i :style="{ width: statPercent(entity.mp, entity.maxMp) + '%' }"></i></div>
            <b>{{ formatStat(entity.mp) }}/{{ formatStat(entity.maxMp) }}</b>
          </div>
          <div class="vital-row st">
            <span>ST</span>
            <div class="vital-bar"><i :style="{ width: statPercent(entity.stamina, entity.maxStamina) + '%' }"></i></div>
            <b>{{ formatStat(entity.stamina) }}/{{ formatStat(entity.maxStamina) }}</b>
          </div>
        </div>

        <div v-if="hasBodyInfo(entity)" class="entity-details">
          <span>H {{ formatHeight(entity.height) }}</span>
          <span>W {{ formatHeight(entity.weight) }}</span>
          <span>U {{ formatHeight(entity.upper) }}</span>
          <span>L {{ formatHeight(entity.lower) }}</span>
        </div>

        <div v-if="hasTitleInfo(entity)" class="entity-titles">
          <span v-if="entity.titleId">T {{ entity.titleId }}</span>
          <span v-if="entity.subTitleId">Sub {{ entity.subTitleId }}</span>
          <span v-if="entity.styleTitleId">Style {{ entity.styleTitleId }}</span>
          <span v-if="entity.styleSubTitleId">StyleSub {{ entity.styleSubTitleId }}</span>
        </div>

        <!-- 公会和基本信息 -->
<!--        <div class="entity-details">-->
<!--          <span v-if="entity.guildName" class="entity-guild">🏠 {{ entity.guildName }}</span>-->
<!--          <span class="entity-height">📏 {{ formatHeight(entity.height) }}</span>-->
<!--        </div>-->

        <!-- 称号信息 -->
<!--        <div class="entity-titles" v-if="entity.titleId || entity.subTitleId">-->
<!--          <span v-if="entity.titleId" class="entity-title">🏅 {{ entity.titleId }}</span>-->
<!--          <span v-if="entity.subTitleId" class="entity-subtitle">📛 {{ entity.subTitleId }}</span>-->
<!--        </div>-->

        <!-- 状态标签 - 使用 Vant Tag -->
        <div v-if="getConditionTags(entity).length > 0" class="entity-conditions">
          <van-tag
            v-for="(tag, index) in getConditionTags(entity)"
            :key="index"
            :type="getTagType(tag.colorClass)"
            size="medium"
            plain
          >
            {{ tag.name }}
          </van-tag>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 实体样式
 * 定义角色列表的样式
 */

.entities-view {
  height: 100%;
}

// 角色条目
.entity-item {
  background: rgba(40, 40, 40, 0.6);
  border-radius: 4px;
  margin-bottom: 3px;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 8px;

  &.self-entity {
    border-left: 3px solid #4caf50;
    background: rgba(76, 175, 80, 0.1);
  }
}

// 角色图标
.entity-icon {
  width: 24px;
  height: 24px;
  border-radius: 4px;
  background: rgba(100, 100, 100, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
}

// 角色信息
.entity-info {
  flex: 1;
}

.entity-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

// 角色名称
.entity-name {
  font-weight: 500;
  color: #fff;
  font-size: 14px;

  &.self-name {
    color: #4caf50 !important;
  }
}

// 自己标签
.self-tag {
  font-size: 10px;
  background: #4caf50;
  color: #fff;
  padding: 1px 4px;
  border-radius: 3px;
  margin-right: 6px;
  font-weight: normal;
}

// 角色种族
.entity-race {
  font-size: 10px;
  color: #888;
}

.entity-vitals {
  margin-top: 5px;
  display: grid;
  gap: 3px;
}

.vital-row {
  display: grid;
  grid-template-columns: 24px 1fr 74px;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: #bbb;

  span {
    color: #888;
    font-weight: 600;
  }

  b {
    text-align: right;
    font-weight: 500;
    color: #ddd;
  }

  .vital-bar {
    height: 5px;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.08);
    overflow: hidden;
  }

  i {
    display: block;
    height: 100%;
    border-radius: inherit;
  }

  &.hp i { background: #ef5350; }
  &.mp i { background: #42a5f5; }
  &.st i { background: #66bb6a; }
}

// 战斗力
.entity-cp {
  font-size: 12px;
  color: #ffc107;
  white-space: nowrap;
}

// 详情
.entity-details {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: #aaa;
  margin-top: 2px;
}

// 公会
.entity-guild {
  color: #4fc3f7;
  font-weight: normal;
  font-size: 12px;
  margin-left: 10px;
}

// 称号
.entity-titles {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: #888;
  margin-top: 2px;
}

.entity-title {
  color: #ffb74d;
}

.entity-subtitle {
  color: #81c784;
}

// 角色状态容器
.entity-conditions {
  margin-top: 4px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  
  :deep(.van-tag) {
    --van-tag-font-size: 9px;
    --van-tag-padding: 2px 6px;
    --van-tag-border-radius: 3px;
  }
}
</style>
