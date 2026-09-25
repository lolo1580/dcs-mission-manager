// Package category classifies DCS unit type identifiers into coarse families
// used to group and icon them on the live map.
//
// DCS type identifiers are exact strings such as "F-16C_50", "T-72B" or
// "SA-10". Classification is heuristic (best effort) and can be overridden per
// type with a JSON file, see Classifier.
package category

import (
	"encoding/json"
	"os"
	"strings"
)

// Known categories.
const (
	Plane     = "plane"
	Heli      = "heli"
	Ground    = "ground"
	Ship      = "ship"
	Structure = "structure"
	Other     = "other"
)

// Classifier maps DCS type identifiers to categories.
type Classifier struct {
	overrides map[string]string
}

// New creates a Classifier. If overridesPath points to a readable JSON file of
// the form {"F-16C_50": "plane", ...}, those entries take precedence over the
// built-in heuristics. Errors are intentionally ignored: classification is
// best effort and must never prevent the backend from starting.
func New(overridesPath string) *Classifier {
	c := &Classifier{}
	if overridesPath == "" {
		return c
	}
	b, err := os.ReadFile(overridesPath)
	if err != nil {
		return c
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return c
	}
	c.overrides = m
	return c
}

// Classify returns the category of a DCS type identifier.
func (c *Classifier) Classify(typeID string) string {
	if typeID == "" {
		return Other
	}
	if c != nil && c.overrides != nil {
		if v, ok := c.overrides[typeID]; ok {
			return v
		}
	}

	// Ships are checked before aircraft: their names rarely overlap, but this
	// keeps the order explicit.
	if hasAnyPrefix(shipPrefixes, typeID) || ships[typeID] {
		return Ship
	}
	// Helicopters before planes, so that "Mi-24" is not caught by a plane rule.
	if hasAnyPrefix(heliPrefixes, typeID) || helis[typeID] {
		return Heli
	}
	if hasAnyPrefix(planePrefixes, typeID) || planes[typeID] {
		return Plane
	}
	if hasAnyPrefix(structurePrefixes, typeID) || structures[typeID] {
		return Structure
	}
	if hasAnyPrefix(groundPrefixes, typeID) || grounds[typeID] {
		return Ground
	}
	return Other
}

func hasAnyPrefix(prefixes []string, s string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

var shipPrefixes = []string{
	"USS_", "HMS_", "CVN", "CV_", "DDG", "FFG", "CG_", "LHA", "LHD", "Type_",
	"Kuznetsov", "Pyotr", "Moskva", "Slava", "Kirov", "Sovremenny", "Udaloy",
	"Grisha", "Molniya", "Tarantul", "Ropucha", "Neustrashimy", "Rezky",
	"Aldaria", "Somers", "Arleigh", "Ticonderoga", "Perry", "Vinson", "Truman",
	"Nimitz", "Stennis", "Roosevelt", "Lincoln", "Washington", "Eisenhower",
	"Bush", "Ford", "Seawise", "Kilo", "Albatros", "Zvezdny", "Yakushev",
}

var heliPrefixes = []string{
	"Mi-8", "Mi-24", "Mi-26", "Mi-28", "Mi-6", "Mi-2",
	"Ka-27", "Ka-50", "Ka-52", "Ka-29",
	"AH-1", "AH-6", "AH-64",
	"UH-1", "UH-60", "SH-60", "CH-47", "CH-53", "OH-58", "MH-60",
	"SA342", "SA 342",
	"NH-90",
}

var planePrefixes = []string{
	"A-10", "A-6", "A-4", "AV-8B", "B-1", "B-2", "B-52", "C-17", "C-130",
	"C-101", "E-3", "E-2", "KC-135", "KC-130", "S-3", "P-51", "F-4", "F-5",
	"F-14", "F-15", "F-16", "F-18", "FA-18", "F-86", "F-117", "F-22",
	"F-100", "F-104", "MiG-", "Su-", "Tu-", "Tu-95", "Tu-160", "An-", "Il-",
	"Yak-", "L-39", "MB-339", "M-2000", "Mirage", "J-11", "JF-17", "J-7",
	"Viggen", "AJS37", "AJ37", "Tornado", "Eurofighter", "Harrier",
	"MQ-9", "RQ-1", "WingLoong", "GlobalHawk", "C-47", "Mosquito",
	"Spitfire", "P-47", "P-40", "FW-190", "Bf-109", "Ju-88", "A-20",
}

var structurePrefixes = []string{
	"FARP", "Oil", "Power", "Warehouse", "Barracks", "Bunker", "Tower",
	"Bridge", "Fuel", "Ammo", "Command", "Control", "Depot", "Factory",
	"Beacon", "ILS", "TACAN", "NDB", "VOR", "Village", "Town", "Airfield",
	"Runway", "Terminal", "Hangar", "Shed", "Tank", "Container",
}

var groundPrefixes = []string{
	"T-", "BTR", "BMP", "BRDM", "MT-LB", "M1", "M2", "M113", "M109", "M270",
	"HMMWV", "Ural", "GAZ", "KAMAZ", "ZIL", "MAZ", "SA-", "S-", "2S", "9K",
	"9A", "9S", "Buk", "Kub", "Tor", "Osa", "Strela", "Tunguska", "Shilka",
	"ZU-23", "ZSU", "Pantsir", "Hawk", "Patriot", "Avenger", "Chaparral",
	"Roland", "Gepard", "Vulcan", "Rapier", "Phalanx", "Abrams", "Leopard",
	"Challenger", "Merkava", "Soldier", "Infantry", "Paratrooper", "MANPADS",
	"Ural", "Land_Rover", "M978", "M818", "M60", "M48", "Truck", "APC",
}

// Exact-match sets complement the prefix rules for types that do not share a
// useful prefix.
var ships = map[string]bool{
	"ARA_Veinticinco_de_Mayo": true,
}

var helis = map[string]bool{}

var planes = map[string]bool{
	"Su-25T": true, "Su-25TM": true, "Su-33": true, "MiG-29S": true,
}

var structures = map[string]bool{}

var grounds = map[string]bool{
	"Grad-URAL": true, "Uragan_BM-27": true, "Smerch": true,
	"2B11_mortar": true, "ZSU-23-4": true,
}
