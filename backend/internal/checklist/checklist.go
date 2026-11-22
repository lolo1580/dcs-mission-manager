// Package checklist contains the checklist model and its validation rules.
//
// A checklist is a named, ordered list of items attached to a flight phase and,
// optionally, to a DCS aircraft type. Built-in templates live in the frontend so
// they follow the interface language; this package only stores the checklists
// the player creates, imports or duplicates.
package checklist

import (
	"errors"
	"fmt"
	"strings"

	"dcsmanager/internal/idgen"
)

// Phases is the fixed vocabulary shared by the interface and the store. Keeping
// it closed lets the UI group and filter checklists reliably.
var Phases = []string{
	"prestart",
	"start",
	"taxi",
	"before_takeoff",
	"takeoff",
	"climb",
	"cruise",
	"descent",
	"approach",
	"landing",
	"after_landing",
	"shutdown",
	"emergency",
}

// Item is one line of a checklist. UID stays stable across edits so that a
// device can keep its progress when the checklist text changes.
type Item struct {
	UID      string `json:"uid"`
	Text     string `json:"text"`
	Response string `json:"response,omitempty"`
	Note     string `json:"note,omitempty"`
	Critical bool   `json:"critical,omitempty"`
}

// Checklist is one stored checklist.
type Checklist struct {
	ID          int64  `json:"id"`
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Aircraft    string `json:"aircraft,omitempty"`
	Phase       string `json:"phase"`
	Description string `json:"description,omitempty"`
	// Source is "user" for a player-made checklist, "import" for one restored
	// from a file and "builtin" for a duplicate of a template. Built-ins
	// themselves are not stored.
	Source string `json:"source"`
	// Visibility is "private" or "squadron". It is used once a server is
	// configured; locally every checklist belongs to the local player.
	Visibility string `json:"visibility"`
	// OwnerID is the server account that owns the checklist; 0 on desktop.
	OwnerID int64 `json:"ownerId,omitempty"`
	// DeletedAt is a synchronisation tombstone.
	DeletedAt int64 `json:"deletedAt,omitempty"`
	// SyncRevision is the server-wide sequence used by pull requests.
	SyncRevision int64 `json:"syncRevision,omitempty"`
	// Revision increments on every save, so a later synchronisation can detect
	// concurrent edits without comparing timestamps only.
	Revision  int64  `json:"revision"`
	Items     []Item `json:"items"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// NewUID returns a random identifier with the given prefix.
func NewUID(prefix string) string { return idgen.New(prefix) }

// ValidPhase reports whether phase belongs to the closed vocabulary.
func ValidPhase(phase string) bool {
	for _, p := range Phases {
		if p == phase {
			return true
		}
	}
	return false
}

// Normalize trims and validates a checklist in place, assigning identifiers to
// new items. requireName is false only for previews that are not stored.
func Normalize(c *Checklist, requireName bool) error {
	c.Name = strings.TrimSpace(c.Name)
	if requireName && c.Name == "" {
		return errors.New("a checklist needs a name")
	}
	if len(c.Name) > 120 {
		return errors.New("the checklist name is too long (120 characters maximum)")
	}
	c.Aircraft = strings.TrimSpace(c.Aircraft)
	if len(c.Aircraft) > 64 {
		return errors.New("the aircraft name is too long (64 characters maximum)")
	}
	c.Phase = strings.TrimSpace(c.Phase)
	if c.Phase == "" {
		c.Phase = "prestart"
	}
	if !ValidPhase(c.Phase) {
		return fmt.Errorf("unknown phase %q", c.Phase)
	}
	c.Description = strings.TrimSpace(c.Description)
	if len(c.Description) > 600 {
		return errors.New("the description is too long (600 characters maximum)")
	}
	switch c.Source {
	case "", "user", "import", "builtin":
		if c.Source == "" {
			c.Source = "user"
		}
	default:
		return fmt.Errorf("unknown source %q", c.Source)
	}
	switch c.Visibility {
	case "":
		c.Visibility = "private"
	case "private", "squadron":
	default:
		return fmt.Errorf("unknown visibility %q", c.Visibility)
	}
	if len(c.Items) == 0 {
		return errors.New("a checklist needs at least one item")
	}
	if len(c.Items) > 200 {
		return errors.New("too many items (200 maximum)")
	}
	seen := make(map[string]struct{}, len(c.Items))
	for i := range c.Items {
		item := &c.Items[i]
		item.Text = strings.TrimSpace(item.Text)
		if item.Text == "" {
			return fmt.Errorf("item %d is empty", i+1)
		}
		if len(item.Text) > 300 {
			return fmt.Errorf("item %d is too long (300 characters maximum)", i+1)
		}
		item.Response = strings.TrimSpace(item.Response)
		if len(item.Response) > 160 {
			return fmt.Errorf("item %d response is too long (160 characters maximum)", i+1)
		}
		item.Note = strings.TrimSpace(item.Note)
		if len(item.Note) > 300 {
			return fmt.Errorf("item %d note is too long (300 characters maximum)", i+1)
		}
		item.UID = strings.TrimSpace(item.UID)
		if item.UID == "" {
			item.UID = NewUID("it_")
		}
		if _, duplicate := seen[item.UID]; duplicate {
			// A file imported twice or edited by hand may repeat an identifier;
			// give the later item a fresh one instead of rejecting the whole
			// checklist.
			item.UID = NewUID("it_")
		}
		seen[item.UID] = struct{}{}
	}
	if c.UID == "" {
		c.UID = NewUID("cl_")
	}
	return nil
}
