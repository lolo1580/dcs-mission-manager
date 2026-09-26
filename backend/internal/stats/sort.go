package stats

import "sort"

// Sorting helpers are kept here so the query code above stays readable.

func sortWeapons(out []WeaponStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kills != out[j].Kills {
			return out[i].Kills > out[j].Kills
		}
		return out[i].Weapon < out[j].Weapon
	})
}

func sortEngines(out []EngineStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kills != out[j].Kills {
			return out[i].Kills > out[j].Kills
		}
		if out[i].Deaths != out[j].Deaths {
			return out[i].Deaths > out[j].Deaths
		}
		return out[i].TypeID < out[j].TypeID
	})
}

func sortCoalitions(out []CoalitionStats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Coalition < out[j].Coalition
	})
}

func sortNetwork(out []NetworkStats) {
	sort.SliceStable(out, func(i, j int) bool { return out[i].AvgPing > out[j].AvgPing })
}
