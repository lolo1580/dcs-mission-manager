package tracker

import "sort"

// sortStats orders sortie analyses by distance flown, longest first.
func sortStats(out []Stats) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DistanceKm != out[j].DistanceKm {
			return out[i].DistanceKm > out[j].DistanceKm
		}
		return out[i].UnitID < out[j].UnitID
	})
}
