package debrief

import (
	"os"
	"testing"
)

const sample = `
mission_file_path	=	".\\Mods\\aircraft\\M-2000C\\Missions\\quickStart/Caucasus M-2000C Free Flight.miz"
callsign	=	"Cellar"
graveyard = 	{}
world_state =
{
	[1] =
	{
		alt	=	321.5,
		type	=	"M-2000C",
		country	=	5,
		y	=	705685.4,
		x	=	-51656.2,
		heading	=	0.52,
		coalition	=	"blue",
		unitId	=	3,
		dead	=	false,
	},
	[2] =
	{
		type	=	"T-72B",
		country	=	2,
		y	=	706467.1,
		x	=	-51600.0,
		coalition	=	"red",
		unitId	=	9,
		dead	=	true,
	},
}
mission_time	=	1913.748
result	=	0
events =
{
	[1] =
	{
		type	=	"mission start",
		event_id	=	17,
		t	=	0,
		linked_event_id	=	0,
	},
	[2] =
	{
		type	=	"takeoff",
		initiatorPilotName	=	"Cellar",
		place	=	"Mineralnye Vody",
		t	=	43.42,
		initiator_unit_type	=	"M-2000C",
		event_id	=	36,
		initiator_coalition	=	2,
	},
	[3] =
	{
		type	=	"kill",
		initiatorPilotName	=	"Cellar",
		target	=	"Bandit",
		weapon	=	"R550 Magic 2",
		t	=	600.5,
		initiator_unit_type	=	"M-2000C",
		event_id	=	40,
	},
	[4] =
	{
		type	=	"land",
		initiatorPilotName	=	"Cellar",
		placeDisplayName	=	"Sukhumi-Babushara",
		t	=	1470.82,
		initiator_unit_type	=	"M-2000C",
		event_id	=	49,
	},
	[5] =
	{
		type	=	"mission end",
		comment	=	"winner: , msg: ",
		t	=	1913.748,
		event_id	=	52,
	},
}
`

func TestParseSample(t *testing.T) {
	d, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if d.Callsign != "Cellar" {
		t.Errorf("callsign = %q", d.Callsign)
	}
	if d.MissionTime != 1913.748 {
		t.Errorf("mission time = %v", d.MissionTime)
	}
	if len(d.WorldState) != 2 {
		t.Fatalf("want 2 world units, got %d", len(d.WorldState))
	}
	if d.WorldState[0].Type != "M-2000C" || d.WorldState[0].Coalition != "blue" {
		t.Errorf("unexpected unit 0: %+v", d.WorldState[0])
	}
	if !d.WorldState[1].Dead {
		t.Errorf("unit 1 should be dead: %+v", d.WorldState[1])
	}

	if len(d.Events) != 5 {
		t.Fatalf("want 5 events, got %d", len(d.Events))
	}
	// Events are sorted by time; the sample is already ordered.
	kill := d.Events[2]
	if kill.Type != "kill" || kill.Weapon != "R550 Magic 2" || kill.Target != "Bandit" {
		t.Errorf("unexpected kill event: %+v", kill)
	}
}

func TestEventsSortedByTime(t *testing.T) {
	d, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i := 1; i < len(d.Events); i++ {
		if d.Events[i].T < d.Events[i-1].T {
			t.Fatalf("events not sorted at %d: %v > %v", i, d.Events[i-1].T, d.Events[i].T)
		}
	}
}

func TestSummarise(t *testing.T) {
	d, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s := d.Summarise()
	if s.Takeoffs != 1 || s.Landings != 1 || s.Kills != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if len(s.Pilots) != 1 || s.Pilots[0] != "Cellar" {
		t.Fatalf("pilots = %v", s.Pilots)
	}
	if s.ByType["mission start"] != 1 || s.ByType["kill"] != 1 {
		t.Fatalf("byType = %v", s.ByType)
	}
}

func TestParseRealDebrief(t *testing.T) {
	path := os.Getenv("DCSMANAGER_TEST_DEBRIEF")
	if path == "" {
		t.Skip("set DCSMANAGER_TEST_DEBRIEF to a debrief.log to run this test")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	d, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(d.Events) == 0 {
		t.Fatal("expected events from a real debrief")
	}
	t.Logf("parsed %d events, %d world units, mission_time=%.1f",
		len(d.Events), len(d.WorldState), d.MissionTime)
	for _, e := range d.Events {
		t.Logf("  t=%.1f %-16s pilot=%q target=%q place=%q",
			e.T, e.Type, e.InitiatorPilot, e.Target, e.PlaceDisplayName)
	}
}
