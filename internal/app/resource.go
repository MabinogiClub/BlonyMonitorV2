package app

import (
	"blonymonitorv2/db"
)

// getSkillNameUnsafe 获取技能名称（需要在锁内调用）。
// 走进程内缓存：该函数会在每个 (target, attacker, skill) 组合上被调用，
// 直接查库会把 CPU 打满并长时间占用读锁，见 resource_cache.go。
func (a *App) getSkillNameUnsafe(skillId int) string {
	return skillNameCache.name(skillId)
}

// getConditionNameUnsafe 获取状态名称（需要在锁内调用）。
func (a *App) getConditionNameUnsafe(conditionId uint32) string {
	return conditionNameCache.name(int(conditionId))
}

// GetSkillName 获取技能名称 (供前端调用)
func (a *App) GetSkillName(skillId int) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.getSkillNameUnsafe(skillId)
}

// GetAllSkillNames 获取所有技能名称映射 (供前端调用)
func (a *App) GetAllSkillNames() map[int]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return db.GetAllSkills()
}

// GetAllConditionNames 获取所有状态名称映射 (供前端调用)
func (a *App) GetAllConditionNames() map[uint32]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	cond_map := db.GetAllCondition()
	cond_map2 := make(map[uint32]string, len(cond_map))
	for k, v := range cond_map {
		cond_map2[uint32(k)] = v
	}
	return cond_map2
}

// GetAllConditionIcons 获取所有状态图标映射 (供前端调用)
func (a *App) GetAllConditionIcons() map[uint32]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	iconMap := db.GetAllConditionIcons()
	result := make(map[uint32]string, len(iconMap))
	for k, v := range iconMap {
		result[uint32(k)] = v
	}
	return result
}

// GetAllSkillIcons 获取所有技能图标映射 (供前端调用)
func (a *App) GetAllSkillIcons() map[int]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return db.GetAllSkillIcons()
}

// GetRegion 获取当前区域 (供前端调用)
func (a *App) GetRegion() string {
	return a.region
}

// SetRegion 设置区域并重新加载资源 (供前端调用)
func (a *App) SetRegion(region string) {
	a.region = region
	// go a.loadResourceData()
}

// ReloadResourceData 重新加载资源数据 (供前端调用)
func (a *App) ReloadResourceData() {
	// go a.loadResourceData()
	println("[ignore] 什么事都不会发生~")
}
