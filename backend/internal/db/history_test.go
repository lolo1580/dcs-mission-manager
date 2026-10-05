package db

import (
	"strconv"
	"testing"

	"dcsmanager/internal/model"
)

func TestEventsSince(t *testing.T) {
	d := openTemp(t)
	id, err := d.EnsureMission("Flight", "Caucasus")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := d.SaveEvent(id, model.Event{Event: "kill", Args: []any{i}, RealTS: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}

	all, err := d.EventsSince(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("EventsSince(0) = %d, want 5", len(all))
	}
	// Oldest first.
	for i := 1; i < len(all); i++ {
		if all[i].ID <= all[i-1].ID {
			t.Fatalf("not ordered oldest first: %+v", all)
		}
	}

	// Strictly greater than the cursor.
	after, err := d.EventsSince(all[2].ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].ID != all[3].ID {
		t.Fatalf("EventsSince(cursor) = %+v, want the two newer events", after)
	}

	// limit caps the batch without skipping (cursor keeps paging correct).
	page, err := d.EventsSince(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 {
		t.Fatalf("limit = %d, want 2", len(page))
	}
	next, err := d.EventsSince(page[len(page)-1].ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || next[0].ID <= page[len(page)-1].ID {
		t.Fatalf("second page = %+v", next)
	}
}

func TestChatSince(t *testing.T) {
	d := openTemp(t)
	for i := 0; i < 3; i++ {
		if err := d.SaveChat(0, model.Chat{From: "p", Message: "m", RealTS: int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := d.ChatSince(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("ChatSince(0) = %d, want 3", len(all))
	}
	after, err := d.ChatSince(all[0].ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("ChatSince(cursor) = %d, want 2", len(after))
	}
}

func TestMissionsSince(t *testing.T) {
	d := openTemp(t)
	for i := 0; i < 3; i++ {
		if _, err := d.StartMission("M"+strconv.Itoa(i), "Caucasus", SourceLive); err != nil {
			t.Fatal(err)
		}
	}
	all, err := d.MissionsSince(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("MissionsSince(0) = %d, want 3", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].ID <= all[i-1].ID {
			t.Fatalf("missions not oldest first: %+v", all)
		}
	}
	after, err := d.MissionsSince(all[0].ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].ID <= all[0].ID {
		t.Fatalf("MissionsSince(cursor) = %+v", after)
	}
}
