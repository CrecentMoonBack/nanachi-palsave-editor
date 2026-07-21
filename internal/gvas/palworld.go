package gvas

// This file holds the Palworld-specific knowledge the generic codec needs:
// which paths carry struct types the wire format does not name, and which
// carry payloads only a game-specific codec can interpret.
//
// Both tables come from palsav (deafdudecomputers/PalworldSaveTools), which is
// the implementation known to read and write saves the game accepts.

// PalworldTypeHints names the struct type used by map keys and values at paths
// where the archive does not state it.
//
// Without a hint the decoder guesses "Guid" for keys and "StructProperty" for
// values, which is right often enough to be dangerous and wrong often enough
// to desynchronise the byte stream.
var PalworldTypeHints = map[string]string{
	".worldSaveData.CharacterContainerSaveData.Key":   "StructProperty",
	".worldSaveData.CharacterContainerSaveData.Value": "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Key":    "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Value":  "StructProperty",
	".worldSaveData.ItemContainerSaveData.Key":        "StructProperty",
	".worldSaveData.ItemContainerSaveData.Value":      "StructProperty",

	".worldSaveData.FoliageGridSaveDataMap.Key":                                        "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value":                                      "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value":                       "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Key":   "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value": "StructProperty",

	".worldSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value": "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":         "StructProperty",

	".worldSaveData.MapObjectSpawnerInStageSaveData.Key":                                                             "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value":                                                           "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Key":                 "Guid",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value":               "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value.ItemMap.Value": "StructProperty",

	".worldSaveData.WorkSaveData.WorkSaveData.WorkAssignMap.Value": "StructProperty",

	".worldSaveData.BaseCampSaveData.Key":                   "Guid",
	".worldSaveData.BaseCampSaveData.Value":                 "StructProperty",
	".worldSaveData.BaseCampSaveData.Value.ModuleMap.Value": "StructProperty",

	".worldSaveData.GroupSaveDataMap.Key":   "Guid",
	".worldSaveData.GroupSaveDataMap.Value": "StructProperty",

	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value":                                       "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value.TreasureBoxInfoMapBySpawnerName.Value": "StructProperty",

	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":         "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value": "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Key":                                             "Guid",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Value":                                           "StructProperty",

	".worldSaveData.InvaderSaveData.Key":                                              "Guid",
	".worldSaveData.InvaderSaveData.Value":                                            "StructProperty",
	".worldSaveData.InvaderDeclarationSaveData.ValidatedStartPointIds.StructProperty": "Guid",

	".worldSaveData.OilrigSaveData.OilrigMap.Value": "StructProperty",

	".worldSaveData.SupplySaveData.SupplyInfos.Key":   "Guid",
	".worldSaveData.SupplySaveData.SupplyInfos.Value": "StructProperty",

	".worldSaveData.GuildExtraSaveDataMap.Key":   "Guid",
	".worldSaveData.GuildExtraSaveDataMap.Value": "StructProperty",

	".SaveData.Local_MaxFriendshipPalIds.Key":   "StructProperty",
	".SaveData.Local_MaxFriendshipPalIds.Value": "StructProperty",
}

// PalworldRawDataPaths lists properties whose ByteProperty payload holds a
// packed binary blob rather than more GVAS structure.
//
// The generic decoder needs no special handling for these — it parses them as
// byte arrays and they round-trip untouched. The list exists so
// internal/palsave knows which blobs are worth interpreting.
var PalworldRawDataPaths = map[string]bool{
	".worldSaveData.GroupSaveDataMap":                                                          true,
	".worldSaveData.CharacterSaveParameterMap.Value.RawData":                                   true,
	".worldSaveData.ItemContainerSaveData.Value.RawData":                                       true,
	".worldSaveData.ItemContainerSaveData.Value.Slots.Slots.RawData":                           true,
	".worldSaveData.CharacterContainerSaveData.Value.Slots.Slots.RawData":                      true,
	".worldSaveData.DynamicItemSaveData.DynamicItemSaveData.RawData":                           true,
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.RawData":                       true,
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value.RawData": true,
	".worldSaveData.BaseCampSaveData.Value.RawData":                                            true,
	".worldSaveData.BaseCampSaveData.Value.WorkerDirector.RawData":                             true,
	".worldSaveData.BaseCampSaveData.Value.WorkCollection.RawData":                             true,
	".worldSaveData.BaseCampSaveData.Value.ModuleMap":                                          true,
	".worldSaveData.WorkSaveData":                                                              true,
	".worldSaveData.MapObjectSaveData":                                                         true,
	".worldSaveData.GuildExtraSaveDataMap.Value.GuildItemStorage.RawData":                      true,
	".worldSaveData.GuildExtraSaveDataMap.Value.Lab.RawData":                                   true,
	".SaveData.WorldMapUISaveDataMap.Value.MaskTextureData":                                    true,
}

// PalworldOptions returns decode options configured for Palworld saves.
func PalworldOptions() *Options {
	return &Options{TypeHints: PalworldTypeHints}
}
