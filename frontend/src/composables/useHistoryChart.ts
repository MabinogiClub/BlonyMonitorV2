import { type Ref } from 'vue'
import { useAppStore } from '../stores/app'
import { getDisplayName, historyTimeToMs } from './useUtils'

type HistoryHitRecord = {
  seq?: number
  damage: number
  rawDamage?: number
  overflowDamage?: number
  adjusted?: boolean
  lockTriggered?: boolean
  lockThreshold?: number
  timestamp: number
  isKill?: boolean
}

type FilteredHistoryHitRecord = HistoryHitRecord & {
  skillId: number
  attackerId: string
  attackerName: string
  skillName: string
}

type KillHistoryHitRecord = HistoryHitRecord & {
  attackerId: string
  skillId: number
  hitIndex: number
}

export function useHistoryChart(targetTimeRange: Ref<{ minTime: number; maxTime: number } | null>, selectedTarget: Ref<any>) {
  const appStore = useAppStore()

  function compareHistoryHits(a: Pick<HistoryHitRecord, 'timestamp' | 'seq'>, b: Pick<HistoryHitRecord, 'timestamp' | 'seq'>): number {
    if (a.timestamp !== b.timestamp) {
      return a.timestamp - b.timestamp
    }
    const aSeq = a.seq ?? Number.MAX_SAFE_INTEGER
    const bSeq = b.seq ?? Number.MAX_SAFE_INTEGER
    return aSeq - bSeq
  }

  function getEffectiveRecordDamage(record: Pick<HistoryHitRecord, 'damage'>): number {
    return Number(record.damage ?? 0)
  }

  function findKillRecordKey(target: any): string | null {
    if (!target || !target.deathTime || target.deathTime <= 0 || !target.attackers) {
      return null
    }

    const deathTime = historyTimeToMs(target.deathTime)
    let killRecord: KillHistoryHitRecord | null = null

    target.attackers.forEach((attacker: any) => {
      if (attacker.isPC === false || !attacker.skillsDetail) return

      attacker.skillsDetail.forEach((skill: any) => {
        if (!skill.hitRecords || skill.hitRecords.length === 0) return

        skill.hitRecords.forEach((record: any, hitIndex: number) => {
          const timestamp = historyTimeToMs(record.timestamp)
          const candidate: KillHistoryHitRecord = {
            seq: record.seq,
            damage: record.damage,
            timestamp,
            attackerId: attacker.id,
            skillId: skill.skillId,
            hitIndex
          }

          if (timestamp > deathTime || getEffectiveRecordDamage(candidate) <= 0) {
            return
          }
          const order = killRecord ? compareHistoryHits(candidate, killRecord) : 1
          if (!killRecord || order > 0 || (order === 0 && candidate.hitIndex > killRecord.hitIndex)) {
            killRecord = candidate
          }
        })
      })
    })

    const finalRecord = killRecord as KillHistoryHitRecord | null
    if (!finalRecord) {
      return null
    }
    return `${finalRecord.attackerId}-${finalRecord.skillId}-${finalRecord.timestamp}-${finalRecord.seq ?? ''}-${finalRecord.hitIndex}`
  }

  function calculateTargetTimeRange(target: any) {
    let minTime = Infinity
    let maxTime = -Infinity

    target.attackers.forEach((attacker: any) => {
      if (attacker.lastHit && historyTimeToMs(attacker.lastHit) > maxTime) {
        maxTime = historyTimeToMs(attacker.lastHit)
      }

      if (!attacker.skillsDetail) return

      attacker.skillsDetail.forEach((skill: any) => {
        if (!skill.hitRecords || skill.hitRecords.length === 0) return

        skill.hitRecords.forEach((record: any) => {
          const timestamp = historyTimeToMs(record.timestamp)
          if (timestamp < minTime) minTime = timestamp
          if (timestamp > maxTime) maxTime = timestamp
        })
      })
    })

    if (target.deathTime !== undefined && target.deathTime > 0) {
      maxTime = historyTimeToMs(target.deathTime)
    }

    if (minTime === Infinity) {
      if (target.appearedAt !== undefined) {
        minTime = historyTimeToMs(target.appearedAt)
      }
    }
    if (maxTime === -Infinity) {
      maxTime = target.appearedAt !== undefined ? historyTimeToMs(target.appearedAt) : minTime
    }

    if (minTime !== Infinity && maxTime !== -Infinity) {
      targetTimeRange.value = { minTime, maxTime }
    } else {
      targetTimeRange.value = null
    }
  }

  function extractChartDataFromHistory(target: any, skillFilters?: Array<{ skillId: number; attackerId?: string }>) {
    if (!target || !target.attackers) {
      appStore.clearHistoryChartData()
      return
    }

    const series: ChartSeries[] = []
    const killRecordKey = findKillRecordKey(target)

    let globalMinTime = targetTimeRange.value?.minTime ?? Infinity
    let globalMaxTime = targetTimeRange.value?.maxTime ?? -Infinity

    if (skillFilters && skillFilters.length > 0) {
      const allSkillRecords: FilteredHistoryHitRecord[] = []

      skillFilters.forEach(filter => {
        const skillName = appStore.skillNameMap[filter.skillId]

        const attackersToProcess = filter.attackerId
          ? target.attackers.filter((a: any) => a.id === filter.attackerId)
          : target.attackers

        attackersToProcess.forEach((attacker: any) => {
          if (attacker.isPC === false) return

          const skill = attacker.skillsDetail?.find((s: any) => s.skillId === filter.skillId)
          if (!skill || !skill.hitRecords || skill.hitRecords.length === 0) return

          const name = getDisplayName(attacker.id, attacker.name)

          skill.hitRecords.forEach((record: any, hitIndex: number) => {
            const timestamp = historyTimeToMs(record.timestamp)
            allSkillRecords.push({
              seq: record.seq,
              damage: record.damage,
              rawDamage: record.rawDamage,
              overflowDamage: record.overflowDamage,
              adjusted: record.adjusted,
              lockTriggered: record.lockTriggered,
              lockThreshold: record.lockThreshold,
              isKill: killRecordKey === `${attacker.id}-${filter.skillId}-${timestamp}-${record.seq ?? ''}-${hitIndex}`,
              timestamp,
              skillId: filter.skillId,
              attackerId: attacker.id,
              attackerName: name,
              skillName: skillName || `技能 #${filter.skillId}`
            })
          })
        })
      })

      const seriesMap = new Map<string, HistoryHitRecord[]>()
      const seriesMeta = new Map<string, { skillId: number; attackerId: string; attackerName: string; skillName: string }>()

      allSkillRecords.forEach(record => {
        const key = `${record.attackerId}-${record.skillId}`
        if (!seriesMap.has(key)) {
          seriesMap.set(key, [])
          seriesMeta.set(key, {
            skillId: record.skillId,
            attackerId: record.attackerId,
            attackerName: record.attackerName,
            skillName: record.skillName
          })
        }
        seriesMap.get(key)!.push({
          seq: record.seq,
          damage: record.damage,
          rawDamage: record.rawDamage,
          overflowDamage: record.overflowDamage,
          adjusted: record.adjusted,
          lockTriggered: record.lockTriggered,
          lockThreshold: record.lockThreshold,
          isKill: record.isKill,
          timestamp: record.timestamp
        })
      })

      seriesMap.forEach((records, key) => {
        const meta = seriesMeta.get(key)!

        records.sort(compareHistoryHits)

        const dataPoints: ChartPoint[] = []
        records.forEach(record => {
          dataPoints.push({
            seq: record.seq,
            time: record.timestamp,
            damage: record.damage,
            singleDamage: record.damage,
            rawDamage: record.rawDamage,
            overflowDamage: record.overflowDamage,
            adjusted: record.adjusted,
            lockTriggered: record.lockTriggered,
            lockThreshold: record.lockThreshold,
            isKill: record.isKill
          } as any)
        })

        if (dataPoints.length > 0) {
          series.push({
            id: meta.attackerId,
            name: `${meta.attackerName} - ${meta.skillName}`,
            data: dataPoints,
            skillId: meta.skillId,
            attackerId: meta.attackerId,
            attackerName: meta.attackerName
          } as any)
        }
      })
    } else {
      target.attackers.forEach((attacker: any) => {
        if (attacker.isPC === false || !attacker.skillsDetail) return

        const allRecords: HistoryHitRecord[] = []

        attacker.skillsDetail.forEach((skill: any) => {
          if (skill.hitRecords && skill.hitRecords.length > 0) {
            skill.hitRecords.forEach((record: any, hitIndex: number) => {
              const timestamp = historyTimeToMs(record.timestamp)
              allRecords.push({
                seq: record.seq,
                damage: record.damage,
                rawDamage: record.rawDamage,
                overflowDamage: record.overflowDamage,
                adjusted: record.adjusted,
                lockTriggered: record.lockTriggered,
                lockThreshold: record.lockThreshold,
                isKill: killRecordKey === `${attacker.id}-${skill.skillId}-${timestamp}-${record.seq ?? ''}-${hitIndex}`,
                timestamp
              })
            })
          }
        })

        if (allRecords.length === 0) return

        allRecords.sort(compareHistoryHits)

        const dataPoints: ChartPoint[] = []
        let cumulativeDamage = 0

        allRecords.forEach(record => {
          cumulativeDamage += record.damage
          dataPoints.push({
            seq: record.seq,
            time: record.timestamp,
            damage: cumulativeDamage,
            singleDamage: record.damage,
            rawDamage: record.rawDamage,
            overflowDamage: record.overflowDamage,
            adjusted: record.adjusted,
            lockTriggered: record.lockTriggered,
            lockThreshold: record.lockThreshold,
            isKill: record.isKill
          } as any)
        })

        const name = getDisplayName(attacker.id, attacker.name)

        if (dataPoints.length > 0) {
          series.push({
            id: attacker.id,
            name: name,
            data: dataPoints,
            attackerId: attacker.id,
            attackerName: name
          } as any)
        }
      })
    }

    if (globalMinTime === Infinity || globalMaxTime === -Infinity || series.length === 0) {
      appStore.clearHistoryChartData()
      return
    }

    if (series.length > 0) {
      appStore.setHistoryChartData(series)
      appStore.chartTimeRange = {
        minTime: globalMinTime,
        maxTime: globalMaxTime
      }
    } else {
      appStore.clearHistoryChartData()
    }
  }

  function handleSkillClick(skillId: number, attackerId?: string) {
    if (!selectedTarget.value) return

    appStore.toggleSkillFilter(skillId, attackerId)

    extractChartDataFromHistory(selectedTarget.value, appStore.selectedSkillFilters)
  }

  function handleClearFilter() {
    if (!selectedTarget.value) return
    extractChartDataFromHistory(selectedTarget.value)
  }

  function handleSkillFilterChanged() {
    if (!selectedTarget.value) return
    extractChartDataFromHistory(selectedTarget.value, appStore.selectedSkillFilters)
  }

  return {
    calculateTargetTimeRange,
    extractChartDataFromHistory,
    handleSkillClick,
    handleClearFilter,
    handleSkillFilterChanged
  }
}
