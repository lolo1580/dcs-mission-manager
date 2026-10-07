package mapping

import (
	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/panel"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestHornetPhysicalTrimDirectionsUsePlugin(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "profiles.json"), nil)
	cat := biosmeta.WithPanelPlugin(&biosmeta.Catalog{Module: "FA-18C_hornet"})
	for _, tc := range []struct {
		cw    bool
		value string
	}{{false, "1"}, {true, "-1"}} {
		ev := panel.Event{Model: panel.PZ70, Control: panel.Control{ID: "PITCH_TRIM", Kind: panel.EncoderPulse, Clockwise: tc.cw}, Active: true}
		got := s.Simulate("FA-18C_hornet", ev, cat)
		if !reflect.DeepEqual(got, []string{"DCSM_PITCH_TRIM " + tc.value + "\n"}) {
			t.Fatalf("physical trim cw=%v: %v", tc.cw, got)
		}
	}
}

func TestHornetWheelDirectionAndRelease(t *testing.T) {
	var sent []string
	var times []time.Time
	s := NewStore(filepath.Join(t.TempDir(), "profiles.json"), func(line string) error { sent = append(sent, line); times = append(times, time.Now()); return nil })
	var hornet Profile
	for _, p := range Starter() {
		if p.Aircraft == "FA-18C_hornet" {
			hornet = p
		}
	}
	if err := s.SetProfile(hornet); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ mode, command string }{{"HDG", "LEFT_DDI_HDG_SW"}, {"CRS", "LEFT_DDI_CRS_SW"}} {
		for _, invert := range []bool{false, true} {
			for i := range hornet.Bindings {
				if hornet.Bindings[i].Control == "LCD_WHEEL" {
					hornet.Bindings[i].Invert = invert
				}
			}
			if err := s.SetProfile(hornet); err != nil {
				t.Fatal(err)
			}
			for _, cw := range []bool{true, false} {
				value := "0"
				if cw != invert {
					value = "2"
				}
				ev := panel.Event{Model: panel.PZ70, Mode: tc.mode, Control: panel.Control{ID: "LCD_WHEEL", Kind: panel.EncoderPulse, Clockwise: cw}, Active: true}
				want := []string{tc.command + " " + value + "\n", tc.command + " 1\n"}
				if got := s.Simulate(hornet.Aircraft, ev, nil); !reflect.DeepEqual(got, want) {
					t.Fatalf("simulate: %v want %v", got, want)
				}
				sent = nil
				times = nil
				if got := s.ApplyForced(hornet.Aircraft, ev, nil); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(sent, want) {
					t.Fatalf("sent: %v", sent)
				}
				if times[1].Sub(times[0]) < 50*time.Millisecond {
					t.Fatal("rocker released before DCS can observe its press")
				}
			}
		}
	}
	ev := panel.Event{Model: panel.PZ70, Mode: "ALT", Control: panel.Control{ID: "LCD_WHEEL", Kind: panel.EncoderPulse}, Active: true}
	if got := s.Simulate(hornet.Aircraft, ev, nil); len(got) != 0 {
		t.Fatalf("heading wheel leaked into ALT: %v", got)
	}
}

func TestHornetMomentaryButtonsAndEngineSelector(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "profiles.json"), nil)
	for _, tc := range []struct {
		model              panel.Model
		id, command, press string
		kind               panel.ControlKind
	}{
		{panel.PZ70, "AP_BUTTON", "UFC_AP", "1", panel.Button},
		{panel.PZ70, "IAS_BUTTON", "THROTTLE_ATC_SW", "1", panel.Button},
		{panel.PZ70, "REV_BUTTON", "STICK_PADDLE_SW", "1", panel.Button},
		{panel.PZ55, "ENGINE_LEFT", "ENGINE_CRANK_SW", "0", panel.Toggle},
		{panel.PZ55, "ENGINE_RIGHT", "ENGINE_CRANK_SW", "2", panel.Toggle},
		{panel.PZ55, "ENGINE_OFF", "ENGINE_CRANK_SW", "1", panel.Toggle},
		{panel.PZ55, "ENGINE_BOTH", "ENGINE_CRANK_SW", "1", panel.Toggle},
	} {
		ev := panel.Event{Model: tc.model, Control: panel.Control{ID: tc.id, Kind: tc.kind}, Active: true}
		if got := s.Simulate("FA-18C_hornet", ev, nil); !reflect.DeepEqual(got, []string{tc.command + " " + tc.press + "\n"}) {
			t.Fatalf("%s press: %v", tc.id, got)
		}
		ev.Active = false
		got := s.Simulate("FA-18C_hornet", ev, nil)
		if tc.kind == panel.Button {
			if !reflect.DeepEqual(got, []string{tc.command + " 0\n"}) {
				t.Fatalf("%s release: %v", tc.id, got)
			}
		} else if len(got) != 0 {
			t.Fatalf("old selector state sent: %v", got)
		}
	}
}
