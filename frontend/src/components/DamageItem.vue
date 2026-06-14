<script setup lang="ts">
/**
 * 伤害条目组件
 * 可复用的伤害统计条目，支持展开/折叠
 */

import { computed } from 'vue'
import { formatNumber, BAR_CLASSES } from '../composables/useUtils'

// 组件属性
const props = defineProps<{
  /** 条目名称 */
  name: string
  /** 伤害值 */
  damage: number
  /** 最大伤害值（用于计算进度条宽度） */
  maxDamage: number
  /** DPS */
  dps?: number
  /** 百分比 */
  percent: number
  /** 颜色索引 */
  colorIndex: number
  /** 是否可展开 */
  expandable?: boolean
  /** 是否已展开 */
  expanded?: boolean
}>()

// 事件
const emit = defineEmits<{
  (e: 'toggle'): void
}>()

/**
 * 进度条宽度
 */
const barWidth = computed(() => {
  return `${(props.damage / props.maxDamage * 100).toFixed(1)}%`
})

/**
 * 进度条样式类
 */
const barClass = computed(() => {
  return BAR_CLASSES[props.colorIndex % BAR_CLASSES.length]
})

/**
 * 处理点击
 */
function handleClick() {
  if (props.expandable) {
    emit('toggle')
  }
}
</script>

<template>
  <div 
    class="damage-item"
    :class="{ 
      expandable: expandable, 
      expanded: expanded 
    }"
    @click="handleClick"
  >
    <!-- 进度条背景 -->
    <div 
      class="damage-bar" 
      :class="barClass"
      :style="{ width: barWidth }"
    ></div>
    
    <!-- 内容 -->
    <div class="damage-content">
      <span class="damage-name">
        <span v-if="expandable" class="expand-icon">▶</span>
        {{ name }}
      </span>
      <div class="damage-info">
        <span v-if="dps !== undefined" class="damage-dps">{{ formatNumber(dps) }}/s</span>
        <span class="damage-value">{{ formatNumber(damage) }}</span>
        <span class="damage-percent">{{ percent.toFixed(1) }}%</span>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
/**
 * 伤害条目样式
 * 定义伤害统计列表的样式
 */

// 伤害条目
.damage-item {
  background: rgba(40, 40, 40, 0.6);
  border-radius: 4px;
  margin-bottom: 3px;
  padding: 6px 8px;
  position: relative;
  overflow: hidden;

  &.expandable {
    cursor: pointer;

    &:hover {
      background: rgba(50, 50, 50, 0.8);
    }
  }

  &.expanded .expand-icon {
    transform: rotate(90deg);
  }
}

// 进度条背景
.damage-bar {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  z-index: 0;
  transition: width 0.3s ease;
}

// 进度条颜色变体
.bar-gold { background: linear-gradient(90deg, rgba(255, 193, 7, 0.5), rgba(255, 193, 7, 0.1)); }
.bar-purple { background: linear-gradient(90deg, rgba(156, 39, 176, 0.4), rgba(156, 39, 176, 0.1)); }
.bar-teal { background: linear-gradient(90deg, rgba(0, 150, 136, 0.4), rgba(0, 150, 136, 0.1)); }
.bar-blue { background: linear-gradient(90deg, rgba(66, 165, 245, 0.4), rgba(66, 165, 245, 0.1)); }
.bar-orange { background: linear-gradient(90deg, rgba(255, 152, 0, 0.4), rgba(255, 152, 0, 0.1)); }
.bar-pink { background: linear-gradient(90deg, rgba(233, 30, 99, 0.4), rgba(233, 30, 99, 0.1)); }

// 伤害内容
.damage-content {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

// 伤害名称
.damage-name {
  font-weight: 500;
  color: #fff;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// 伤害信息
.damage-info {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 11px;
}

// 伤害值
.damage-value {
  font-weight: 600;
  color: #ffc107;
  min-width: 50px;
  text-align: right;
}

// 百分比
.damage-percent {
  color: #aaa;
  min-width: 36px;
  text-align: right;
}

// DPS
.damage-dps {
  color: #81c784;
  min-width: 50px;
  text-align: right;
  font-size: 10px;
}

// 展开图标
.expand-icon {
  margin-right: 4px;
  font-size: 10px;
  transition: transform 0.2s;
}

// 子条目容器
.sub-items {
  display: none;
  margin-left: 12px;
  margin-top: 3px;
}

.expanded + .sub-items {
  display: block;
}

// 子条目
.sub-item {
  background: rgba(35, 35, 35, 0.6);
  border-radius: 3px;
  margin-bottom: 2px;
  padding: 4px 8px;
  font-size: 11px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: relative;
  overflow: hidden;
}

// 子条目进度条
.sub-item-bar {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  z-index: 0;
  opacity: 0.5;
}

// 子条目内容
.sub-item-content {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

// 子条目名称
.sub-item-name {
  color: #ccc;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// 子条目统计
.sub-item-stats {
  display: flex;
  gap: 6px;
  font-size: 10px;
  color: #888;
}

// 子条目伤害值
.sub-item-damage {
  color: #ffc107;
}

// 命中次数
.damage-hits {
  color: #90caf9;
  font-size: 10px;
}
</style>
