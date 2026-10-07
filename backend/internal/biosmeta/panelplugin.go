package biosmeta

// WithPanelPlugin returns a separately indexed catalogue, leaving the BIOS
// metadata and its LED sources intact. Only the verified Hornet is supported.
func WithPanelPlugin(c *Catalog) *Catalog {
	if c == nil || c.Module != "FA-18C_hornet" {
		return c
	}
	raw := make(map[string]map[string]Control)
	for _, ctl := range c.Controls {
		if raw[ctl.Category] == nil {
			raw[ctl.Category] = make(map[string]Control)
		}
		raw[ctl.Category][ctl.Identifier] = ctl
	}
	raw["DCS Manager plugin"] = map[string]Control{"DCSM_PITCH_TRIM": {
		Identifier: "DCSM_PITCH_TRIM", ControlType: "encoder",
		Description: "Trim longitudinal — plugin DCS Manager (à valider en cockpit)",
		Inputs:      []Input{{Interface: "variable_step", SuggestedStep: 1}},
	}}
	return newCatalog(c.Module, raw)
}
