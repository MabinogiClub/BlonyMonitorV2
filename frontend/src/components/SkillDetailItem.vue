<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/app'
import { formatNumber, getSkillName, getSkillIconUrl, BAR_CLASSES } from '../composables/useUtils'

interface SkillStats {
  skillId: number
  totalDamage: number
  hitCount: number
  critCount: number
  avgDamage: number
  minDamage: number
  maxDamage: number
  critMinDamage?: number
  critMaxDamage?: number
  percent: number
}

const props = defineProps<{
  skill: SkillStats
  skillIndex: number
  parentIndex: number
  maxSkillDamage: number
  attackerId?: string
}>()

const emit = defineEmits<{
  clickSkill: [skillId: number, attackerId?: string]
}>()

const appStore = useAppStore()

const isSelected = computed(() =>
  appStore.selectedSkillFilters.some(
    f => f.skillId === props.skill.skillId && f.attackerId === props.attackerId
  )
)

function getBarClass(index: number): string {
  return BAR_CLASSES[index % BAR_CLASSES.length]
}

function getDisplayDamageRange(skill: SkillStats): { min: number; max: number } {
  const allMins = [skill.minDamage, skill.critMinDamage ?? 0].filter(v => v > 0)
  const allMaxs = [skill.maxDamage, skill.critMaxDamage ?? 0].filter(v => v > 0)
  return {
    min: allMins.length ? Math.min(...allMins) : 0,
    max: allMaxs.length ? Math.max(...allMaxs) : 0,
  }
}
</script>

<template>
  <div class="sub-item history-sub-row" :class="{ selected: isSelected }">
    <div
      class="sub-item-bar"
      :class="getBarClass((parentIndex + skillIndex) % BAR_CLASSES.length)"
      :style="{ width: `${(skill.totalDamage / (maxSkillDamage || 1) * 100).toFixed(1)}%` }"
    />
    <div class="sub-item-content">
      <div class="sub-item-name">
        <img
          class="skill-icon"
          :src="getSkillIconUrl(skill.skillId)"
          alt=""
          width="18"
          height="18"
          @error="($event.target as HTMLImageElement).style.display = 'none'"
        >
        <span
          class="skill-name-clickable"
          :class="{ active: isSelected }"
          @click.stop="emit('clickSkill', skill.skillId, attackerId)"
        >{{ getSkillName(skill.skillId) }}</span>
      </div>
      <div class="sub-item-stats">
        <span>{{ skill.hitCount }}次</span>
        <span>{{ skill.critCount }}暴击</span>
        <span class="hover-tip" :data-tooltip="skill.avgDamage.toLocaleString()">
          平均{{ formatNumber(skill.avgDamage) }}
        </span>
        <span
          class="hover-tip"
          :data-tooltip="`${getDisplayDamageRange(skill).min.toLocaleString()} ~ ${getDisplayDamageRange(skill).max.toLocaleString()}`"
        >
          {{ formatNumber(getDisplayDamageRange(skill).min) }}~{{ formatNumber(getDisplayDamageRange(skill).max) }}
        </span>
        <span class="hover-tip sub-item-damage" :data-tooltip="skill.totalDamage.toLocaleString()">
          {{ formatNumber(skill.totalDamage) }}
        </span>
        <span>{{ skill.percent.toFixed(1) }}%</span>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.sub-item-name {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
}

.skill-icon {
  flex-shrink: 0;
  border-radius: 2px;
  display: block;
}

.skill-name-clickable {
  cursor: pointer;
  user-select: none;
  line-height: 18px;
  transition: color 0.2s;

  &:hover,
  &.active {
    color: #42a5f5;
  }
}

.history-sub-row.selected {
  background: rgba(66, 165, 245, 0.12);
}

.sub-item-stats {
  flex-shrink: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
}
</style>
