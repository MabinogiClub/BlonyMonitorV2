package app

import "blonymonitorv2/db"

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
		if mapInfo.MapName != "" {
			return mapInfo.MapName
		}
	}
	if mapInfo.LocalName != "" && mapInfo.LocalName != "???" {
		return mapInfo.LocalName
	}
	if mapInfo.MapName != "" && mapInfo.MapName != "???" {
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

	if a.currentInstance != nil && a.currentInstance.InstanceName != "" &&
		(a.currentInstance.MapID != 0 || a.currentMapIsInstanceUnsafe()) {
		if a.instanceSaveName != "" {
			return a.instanceSaveName
		}
		return a.currentInstance.InstanceName
	}

	return mapSaveName(a.currentMap, fallback)
}

func (a *App) transitionSaveNameUnsafe(fallback string, oldMapID int) string {
	saveName := a.currentSaveNameUnsafe(fallback)
	if saveName != fallback {
		return saveName
	}

	if isInstanceMapID(oldMapID) {
		mapInfo := db.NewMinimapInfo_FieldMapInfoList(oldMapID)
		if mapInfo.MapLocalName != "" {
			return mapInfo.MapLocalName
		}
	}
	return saveName
}
