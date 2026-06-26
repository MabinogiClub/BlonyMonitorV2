package app

import "blonymonitorv2/db"

func isGenericSaveName(name string) bool {
	return name == "" || name == "???" || name == defaultInstanceDisplayName || name == "战斗记录"
}

func finalizeSaveName(name string) string {
	if isGenericSaveName(name) {
		return "战斗记录"
	}
	return name
}

func localizedDungeonSaveName(internalName string) string {
	if internalName == "" {
		return ""
	}

	dungeonInfo := db.NewDungeonDB(internalName)
	if dungeonInfo.LocalName != "" {
		return dungeonInfo.LocalName
	}
	return internalName
}

func mapSaveName(mapInfo *CurrentMapInfo, fallback string) string {
	if mapInfo == nil {
		return fallback
	}

	if mapInfo.LocalName == "地下城" || mapInfo.LocalName == defaultInstanceDisplayName {
		if mapInfo.MapName != "" && !isGenericSaveName(mapInfo.MapName) {
			return mapInfo.MapName
		}
	}
	if mapInfo.LocalName != "" && !isGenericSaveName(mapInfo.LocalName) {
		return mapInfo.LocalName
	}
	if mapInfo.MapName != "" && !isGenericSaveName(mapInfo.MapName) {
		return mapInfo.MapName
	}
	return fallback
}

func (a *App) currentSaveNameUnsafe(fallback string) string {
	if a.currentDungeon != nil {
		if a.dungeonSaveName != "" {
			return a.dungeonSaveName
		}
		if saveName := localizedDungeonSaveName(a.currentDungeon.DungeonName); saveName != "" {
			return saveName
		}
	}

	if a.instanceSaveName != "" && !isGenericSaveName(a.instanceSaveName) &&
		(a.currentMapIsInstanceUnsafe() || a.currentInstance != nil || isInstanceMapID(a.instanceEnterMapID)) {
		return a.instanceSaveName
	}

	if a.currentInstance != nil && a.currentInstance.InstanceName != "" &&
		(a.currentInstance.MapID != 0 || a.currentMapIsInstanceUnsafe()) {
		if a.instanceSaveName != "" && !isGenericSaveName(a.instanceSaveName) {
			return a.instanceSaveName
		}
		return a.currentInstance.InstanceName
	}

	return mapSaveName(a.currentMap, fallback)
}

func (a *App) transitionSaveNameUnsafe(fallback string, oldMapID int, instanceSaveName string) string {
	saveName := finalizeSaveName(a.currentSaveNameUnsafe(fallback))
	if !isGenericSaveName(saveName) {
		return saveName
	}

	if instanceSaveName != "" && !isGenericSaveName(instanceSaveName) {
		return instanceSaveName
	}

	if isInstanceMapID(oldMapID) {
		mapInfo := db.NewMinimapInfo_FieldMapInfoList(oldMapID)
		if mapInfo.MapLocalName != "" && !isGenericSaveName(mapInfo.MapLocalName) {
			return mapInfo.MapLocalName
		}
	}
	return "战斗记录"
}
