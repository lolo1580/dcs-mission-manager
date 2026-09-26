// Package visibility implements the mission's F10 view options ("fog of war").
//
// DCS lets a mission author restrict what the F10 map reveals. A live map that
// ignores those options would leak information the mission deliberately hides,
// which is both a gameplay and an integrity problem on multiplayer servers.
//
// The policy here is deliberately *restrictive*: it never shows more than DCS
// itself would. The mission option values come from DCS's own options database
// (MissionEditor/modules/Options/optionsDb.lua):
//
//	optview_onlymap      "MAP ONLY"     no units
//	optview_myaircraft   "MY A/C"       own aircraft only
//	optview_allies       "FOG OF WAR"   allies plus sensor contacts
//	optview_onlyallies   "ALLIES ONLY"  allies only
//	optview_all          "ALL"          everything
//
// "FOG OF WAR" additionally depends on each coalition's live sensor detections,
// which the Lua export does not provide. Rather than guess, this package treats
// it as "allies only" — the safe subset.
package visibility

import (
	"strings"
	"sync"

	"dcsmm/internal/state"
)

// Mode is the mission's F10 view option.
type Mode string

const (
	// MapOnly: no unit symbols at all.
	MapOnly Mode = "map_only"
	// MyAircraft: only the player's own aircraft.
	MyAircraft Mode = "my_aircraft"
	// Allies: fog of war (allies plus sensor contacts); treated as allies-only.
	Allies Mode = "allies"
	// OnlyAllies: allies only, no enemy contacts.
	OnlyAllies Mode = "only_allies"
	// All: everything is visible.
	All Mode = "all"
	// Unknown: no mission options received yet. Treated as Allies for safety.
	Unknown Mode = "unknown"
)

// FromDCSOption maps a DCS `optionsView` value to a Mode.
func FromDCSOption(value string) Mode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "optview_onlymap":
		return MapOnly
	case "optview_myaircraft":
		return MyAircraft
	case "optview_allies":
		return Allies
	case "optview_onlyallies":
		return OnlyAllies
	case "optview_all":
		return All
	default:
		return Unknown
	}
}

// Label is a human-readable French label for the mode.
func (m Mode) Label() string {
	switch m {
	case MapOnly:
		return "Carte seule"
	case MyAircraft:
		return "Mon appareil"
	case Allies:
		return "Fog of war (alliés)"
	case OnlyAllies:
		return "Alliés uniquement"
	case All:
		return "Tout"
	default:
		return "Inconnu"
	}
}

// Policy holds the current visibility mode and filters units accordingly.
type Policy struct {
	mu       sync.RWMutex
	mode     Mode
	override bool
}

// New creates a policy. With override true, filtering is disabled entirely
// (for solo use and mission design, where the mission author *is* the user).
func New(override bool) *Policy {
	return &Policy{mode: Unknown, override: override}
}

// SetMode updates the current mode.
func (p *Policy) SetMode(m Mode) {
	p.mu.Lock()
	p.mode = m
	p.mu.Unlock()
}

// Mode returns the current mode.
func (p *Policy) Mode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// Override reports whether filtering is disabled.
func (p *Policy) Override() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.override
}

// Filter returns the subset of units that may be shown.
//
// The ownship is always kept: showing your own position is never a leak, and
// without it the map would be unusable in the restrictive modes.
func (p *Policy) Filter(units []state.Unit) []state.Unit {
	p.mu.RLock()
	mode := p.mode
	override := p.override
	p.mu.RUnlock()

	if override || mode == All {
		return units
	}

	// Resolve the player's coalition from the ownship.
	ownSide := ""
	for _, u := range units {
		if u.Ownship {
			ownSide = u.Coalition
			break
		}
	}

	out := make([]state.Unit, 0, len(units))
	for _, u := range units {
		if p.allowed(u, mode, ownSide) {
			out = append(out, u)
		}
	}
	return out
}

func (p *Policy) allowed(u state.Unit, mode Mode, ownSide string) bool {
	// Your own aircraft is always visible.
	if u.Ownship {
		return true
	}
	switch mode {
	case MapOnly, MyAircraft:
		return false
	case Allies, OnlyAllies:
		// Own coalition only. Neutral units are not shown: DCS does not reveal
		// them without a detection either.
		return ownSide != "" && u.Coalition == ownSide
	case Unknown:
		// No options received: behave like the restrictive "fog of war" case.
		return ownSide != "" && u.Coalition == ownSide
	default:
		return false
	}
}

// Description summarises the active filtering, for the UI and logs.
type Description struct {
	Mode     Mode   `json:"mode"`
	Label    string `json:"label"`
	Override bool   `json:"override"`
	Note     string `json:"note,omitempty"`
}

// Describe returns the current policy as a serialisable description.
func (p *Policy) Describe() Description {
	mode := p.Mode()
	d := Description{
		Mode:     mode,
		Label:    mode.Label(),
		Override: p.Override(),
	}
	if p.Override() {
		d.Note = "filtrage désactivé : toutes les unités sont diffusées"
		return d
	}
	switch mode {
	case Allies:
		d.Note = "les contacts détectés par les capteurs ne sont pas reproduits (restrictif)"
	case Unknown:
		d.Note = "options de mission pas encore reçues ; filtrage restrictif appliqué"
	}
	return d
}
