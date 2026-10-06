package biosmeta

import (
	"fmt"
	"strings"
)

// Interface is a DCS-BIOS command interface.
type Interface string

const (
	// FixedStep steps a selector to its next or previous position.
	FixedStep Interface = "fixed_step"
	// SetState jumps a selector or dial to an absolute position.
	SetState Interface = "set_state"
	// VariableStep nudges a dial by a step.
	VariableStep Interface = "variable_step"
	// Action triggers a momentary control.
	Action Interface = "action"
)

// Command is one DCS-BIOS command to send: an identifier and an argument.
type Command struct {
	Identifier string
	Value      int
}

// Line renders the command the way DCS-BIOS expects it on the wire.
func (c Command) Line() string {
	return fmt.Sprintf("%s %d\n", c.Identifier, c.Value)
}

// ArgForInterface returns the argument DCS-BIOS expects for an interface given
// the control's declared maximum.
//
// It is the form a mapping uses: the mapping stores an interface name, not the
// whole control, so that a profile stays valid when DCS-BIOS metadata changes.
func ArgForInterface(iface Interface, maxValue int, active bool) (int, bool) {
	switch iface {
	case FixedStep:
		// INC is 0, DEC is 2 in DCS-BIOS.
		if active {
			return 0, true
		}
		return 2, true
	case SetState:
		if maxValue <= 0 {
			// No declared maximum: the control only has two positions.
			if active {
				return 1, true
			}
			return 0, true
		}
		if active {
			return maxValue, true
		}
		return 0, true
	case VariableStep:
		if active {
			return 1, true
		}
		return -1, true
	case Action:
		if active {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// ArgForInput returns the argument DCS-BIOS expects for a given interface, using
// the control's own declared maximum and suggested step.
//
// The conventions come from DCS-BIOS' own control modules:
//
//   - FixedStep: 0 or 1 steps one way, 2 the other. The panel protocol reports a
//     two-state switch, so "on" steps forward and "off" steps back.
//   - SetState: the absolute position, clamped to the control's maximum.
//   - Action: 1 presses, 0 releases.
//
// An interface the control does not accept yields ok=false: a mapping must not
// send a command the aircraft would log as invalid.
func ArgForInput(ctl Control, iface Interface, active bool) (int, bool) {
	if !ctl.Supports(string(iface)) {
		return 0, false
	}
	max := 0
	for _, in := range ctl.Inputs {
		if in.Interface == string(SetState) && in.MaxValue > max {
			max = in.MaxValue
		}
	}
	value, ok := ArgForInterface(iface, max, active)
	if !ok {
		return 0, false
	}
	// SetState is the only one where the value is a real position; the others are
	// fixed by convention above.
	if iface == VariableStep {
		step := 1
		for _, in := range ctl.Inputs {
			if in.Interface == string(VariableStep) && in.SuggestedStep > 0 {
				step = in.SuggestedStep
			}
		}
		if active {
			return step, true
		}
		return -step, true
	}
	return value, true
}

// ArgForEncoder returns the argument for one detent of a rotating control, whose
// direction decides the sign.
//
// Unlike a switch, both directions of a wheel produce the same "active" edge, so
// the sign must come from the direction. DCS-BIOS convention: variable_step is
// +step turning one way and -step the other; fixed_step is 0 (INC) and 2 (DEC).
// An interface the control does not accept yields ok=false.
func ArgForEncoder(iface Interface, suggestedStep, maxValue int, clockwise bool) (int, bool) {
	switch iface {
	case VariableStep:
		step := suggestedStep
		if step <= 0 {
			step = 1
		}
		if clockwise {
			return step, true
		}
		return -step, true
	case FixedStep:
		if clockwise {
			return 0, true
		}
		return 2, true
	}
	// Anything else (a rotary selector) has no inherent direction: fall back to the
	// switch convention.
	return ArgForInterface(iface, maxValue, clockwise)
}

// DefaultInterface picks the command interface best suited to a control, so a
// mapping editor can suggest one instead of making the user choose among four.
//
// A momentary control wants Action; a two-state panel switch driving a selector
// wants SetState, which is idempotent; anything else falls back to the first
// interface the control declares.
func DefaultInterface(ctl Control) (Interface, bool) {
	if !ctl.Writable() {
		return "", false
	}
	switch {
	case ctl.Supports(string(Action)):
		return Action, true
	case ctl.Supports(string(SetState)):
		return SetState, true
	case ctl.Supports(string(FixedStep)):
		return FixedStep, true
	case ctl.Supports(string(VariableStep)):
		return VariableStep, true
	}
	// Fall back to whatever it declares, so an unknown interface still works.
	return Interface(ctl.Inputs[0].Interface), true
}

// Validate checks that a mapping's interface is one the control actually accepts,
// so an invalid mapping is refused when it is saved rather than when it is sent.
func Validate(ctl Control, iface Interface) error {
	if !ctl.Writable() {
		return fmt.Errorf("%s is read-only: it has no command", ctl.Identifier)
	}
	if iface == "" {
		return fmt.Errorf("%s: no interface chosen", ctl.Identifier)
	}
	if !ctl.Supports(string(iface)) {
		var have []string
		for _, in := range ctl.Inputs {
			have = append(have, in.Interface)
		}
		return fmt.Errorf("%s does not accept %q (it accepts %s)",
			ctl.Identifier, iface, strings.Join(have, ", "))
	}
	return nil
}
