package app

import "time"

const (
	instanceMapIDMin = 35000
	instanceMapIDMax = 35999

	instanceNameWaitCentis        int64 = 300
	pendingInstanceNameTTL        int64 = 500
	instanceNameDisplayWaitMillis int64 = 100
	defaultInstanceDisplayName          = "副本"
)

func isInstanceMapID(mapID int) bool {
	return mapID >= instanceMapIDMin && mapID <= instanceMapIDMax
}

func (a *App) currentMapIsInstanceUnsafe() bool {
	return a.currentMap != nil && isInstanceMapID(a.currentMap.MapID)
}

func (a *App) instanceNameTargetMapIDUnsafe(nowCentis int64) (int, bool, bool) {
	if a.currentMapIsInstanceUnsafe() {
		return a.currentMap.MapID, true, false
	}

	waiting := isInstanceMapID(a.instanceNameWaitMapID) &&
		a.instanceNameWaitUntil > 0 &&
		nowCentis <= a.instanceNameWaitUntil
	if waiting {
		return a.instanceNameWaitMapID, false, true
	}

	return 0, false, false
}

func (a *App) startInstanceNameWaitUnsafe(mapID int, nowCentis int64) {
	a.instanceNameWaitMapID = mapID
	a.instanceNameWaitUntil = nowCentis + instanceNameWaitCentis
}

func (a *App) clearInstanceNameWaitUnsafe() {
	a.instanceNameWaitMapID = 0
	a.instanceNameWaitUntil = 0
}

func (a *App) resetInstanceBindingUnsafe() {
	a.currentInstance = nil
	a.instanceEnterMapID = 0
	a.instanceNameReceived = false
	a.clearInstanceNameWaitUnsafe()
}

func (a *App) resetInstanceStateUnsafe() {
	a.resetInstanceBindingUnsafe()
	a.instanceSaveName = ""
}

func (a *App) restoreInstanceBindingFromSaveNameUnsafe(mapID int, nowCentis int64) bool {
	if a.instanceSaveName == "" || isGenericSaveName(a.instanceSaveName) {
		return false
	}
	a.currentInstance = &InstanceInfo{
		InstanceName: a.instanceSaveName,
		MapID:        0,
		EnteredAt:    nowCentis,
	}
	_, ok := a.bindCurrentInstanceToMapUnsafe(mapID)
	return ok
}

func (a *App) clearExpiredPendingInstanceUnsafe(nowCentis int64) {
	if a.currentInstance == nil || a.currentInstance.MapID != 0 {
		return
	}
	if nowCentis-a.currentInstance.EnteredAt <= pendingInstanceNameTTL {
		return
	}

	logger.Printf("[Instance] 清除过期的待绑定副本名称: %s\n", a.currentInstance.InstanceName)
	a.resetInstanceStateUnsafe()
}

func (a *App) bindCurrentInstanceToMapUnsafe(mapID int) (string, bool) {
	if a.currentInstance == nil || a.currentInstance.InstanceName == "" {
		return "", false
	}

	a.currentInstance.MapID = uint32(mapID)
	if a.instanceEnterMapID == 0 {
		a.instanceEnterMapID = mapID
		logger.Printf("[Instance] 记录进入副本时的地图ID: %d\n", mapID)
	}
	a.clearInstanceNameWaitUnsafe()

	return a.currentInstance.InstanceName, true
}

func (a *App) resolveInstanceMapDisplay(mapID int) (string, string) {
	logger.Printf("[Instance Debug] 检测到副本地图ID: %d，检查 currentInstance...\n", mapID)

	a.mu.Lock()
	nowCentis := time.Now().UnixMilli() / 10
	a.clearExpiredPendingInstanceUnsafe(nowCentis)
	a.startInstanceNameWaitUnsafe(mapID, nowCentis)
	if a.currentInstance != nil && a.currentInstance.MapID != 0 && int(a.currentInstance.MapID) != mapID {
		logger.Printf("[Instance] 检测到新的副本地图 (%d -> %d)，保留副本名称等待重新绑定\n", a.currentInstance.MapID, mapID)
		a.resetInstanceBindingUnsafe()
		a.restoreInstanceBindingFromSaveNameUnsafe(mapID, nowCentis)
	}
	if a.currentInstance != nil {
		logger.Printf("[Instance Debug] currentInstance存在: %s\n", a.currentInstance.InstanceName)
		if instanceName, ok := a.bindCurrentInstanceToMapUnsafe(mapID); ok {
			a.mu.Unlock()
			logger.Printf("[Instance] 使用副本名称: %s\n", instanceName)
			return instanceName, defaultInstanceDisplayName
		}

		a.mu.Unlock()
		logger.Printf("[Instance Debug] currentInstance名称为空\n")
		return defaultInstanceDisplayName, defaultInstanceDisplayName
	}
	a.mu.Unlock()

	logger.Printf("[Instance] 副本名称未到达，等待%dms...\n", instanceNameDisplayWaitMillis)
	time.Sleep(time.Duration(instanceNameDisplayWaitMillis) * time.Millisecond)

	a.mu.Lock()
	if a.currentInstance != nil && a.currentInstance.InstanceName != "" &&
		(a.currentInstance.MapID == 0 || int(a.currentInstance.MapID) == mapID) {
		instanceName, _ := a.bindCurrentInstanceToMapUnsafe(mapID)
		a.mu.Unlock()
		logger.Printf("[Instance] 延迟后获取到副本名称: %s\n", instanceName)
		return instanceName, defaultInstanceDisplayName
	}
	if a.restoreInstanceBindingFromSaveNameUnsafe(mapID, time.Now().UnixMilli()/10) {
		instanceName := a.instanceSaveName
		a.mu.Unlock()
		logger.Printf("[Instance] 延迟后使用已保存副本名称: %s\n", instanceName)
		return instanceName, defaultInstanceDisplayName
	}

	a.mu.Unlock()
	logger.Printf("[Instance] 延迟后仍未获取到副本名称，使用默认名称\n")
	return defaultInstanceDisplayName, defaultInstanceDisplayName
}
