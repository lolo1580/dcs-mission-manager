package main

import (
	"context"
	"log"
	"time"
)

// snapshotKinds are the /api/stats/<kind> endpoints polled on every pass.
var snapshotKinds = []string{"overview", "pilots", "weapons", "engines", "network"}

// Syncer polls the manager and writes snapshots to Postgres.
type Syncer struct {
	cfg    Config
	client *ManagerClient
	store  *Store
}

// NewSyncer wires a syncer.
func NewSyncer(cfg Config, client *ManagerClient, store *Store) *Syncer {
	return &Syncer{cfg: cfg, client: client, store: store}
}

// Run polls once immediately, then on every interval until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	s.tick(ctx)

	ticker := time.NewTicker(s.cfg.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs one pass and records its outcome. A failure (the manager not being
// up yet, for instance) is logged and retried on the next interval.
func (s *Syncer) tick(ctx context.Context) {
	if err := s.once(ctx); err != nil {
		log.Printf("sync: %v", err)
		_ = s.store.RecordSync(ctx, s.cfg.Name, false, err.Error())
		return
	}
	_ = s.store.RecordSync(ctx, s.cfg.Name, true, "")
}

func (s *Syncer) once(ctx context.Context) error {
	if err := s.snapshot(ctx); err != nil {
		return err
	}
	if s.cfg.Mirror {
		if err := s.mirror(ctx); err != nil {
			return err
		}
	}
	return nil
}

// snapshot polls the aggregate stats endpoints and stores a snapshot of each.
func (s *Syncer) snapshot(ctx context.Context) error {
	inserted := 0
	for _, scope := range s.cfg.Scopes {
		for _, kind := range snapshotKinds {
			body, err := s.client.StatsRaw(ctx, kind, scope, s.cfg.IncludeTest)
			if err != nil {
				return err
			}
			ok, err := s.store.InsertSnapshot(ctx, s.cfg.Name, kind, scope, s.cfg.IncludeTest, body)
			if err != nil {
				return err
			}
			if ok {
				inserted++
			}
		}
	}
	log.Printf("sync: %d new snapshot(s)", inserted)
	return nil
}

// mirror incrementally copies events, chat and missions into PostgreSQL, using
// the manager's ?sinceId= feed. It loops until a page comes back shorter than
// the batch, so a large backlog is caught up in one pass.
func (s *Syncer) mirror(ctx context.Context) error {
	total := 0
	n, err := s.pull(ctx, "events")
	if err != nil {
		return err
	}
	total += n
	n, err = s.pull(ctx, "chat")
	if err != nil {
		return err
	}
	total += n
	n, err = s.pull(ctx, "missions")
	if err != nil {
		return err
	}
	total += n

	// The incremental feed only sees NEW missions. A mission that ends keeps its
	// id but gains ended_at/winner, so the most recent missions are refreshed to
	// capture those updates (they are few: at most MirrorBatch).
	if err := s.refreshRecentMissions(ctx); err != nil {
		return err
	}

	if total > 0 {
		log.Printf("mirror: %d new row(s)", total)
	}
	return nil
}

// refreshRecentMissions re-mirrors the newest missions so that changes to an
// existing mission (its end, its winner) are picked up. Idempotent upserts make
// this cheap and safe to run every pass.
func (s *Syncer) refreshRecentMissions(ctx context.Context) error {
	missions, err := s.client.RecentMissions(ctx, s.cfg.MirrorBatch)
	if err != nil {
		return err
	}
	if len(missions) == 0 {
		return nil
	}
	_, err = s.store.UpsertMissions(ctx, s.cfg.Name, missions)
	return err
}

// pull copies one feed ("events", "chat" or "missions") until it is caught up.
func (s *Syncer) pull(ctx context.Context, name string) (int, error) {
	total := 0
	for {
		cursor, err := s.store.Cursor(ctx, s.cfg.Name, name)
		if err != nil {
			return total, err
		}

		var (
			got  int
			next int64
		)
		switch name {
		case "events":
			rows, n, err := s.client.EventsSince(ctx, cursor, s.cfg.MirrorBatch)
			if err != nil {
				return total, err
			}
			got, next = len(rows), n
			if got > 0 {
				if _, err := s.store.UpsertEvents(ctx, s.cfg.Name, rows); err != nil {
					return total, err
				}
			}
		case "chat":
			rows, n, err := s.client.ChatSince(ctx, cursor, s.cfg.MirrorBatch)
			if err != nil {
				return total, err
			}
			got, next = len(rows), n
			if got > 0 {
				if _, err := s.store.UpsertChat(ctx, s.cfg.Name, rows); err != nil {
					return total, err
				}
			}
		case "missions":
			rows, n, err := s.client.MissionsSince(ctx, cursor, s.cfg.MirrorBatch)
			if err != nil {
				return total, err
			}
			got, next = len(rows), n
			if got > 0 {
				if _, err := s.store.UpsertMissions(ctx, s.cfg.Name, rows); err != nil {
					return total, err
				}
			}
		}

		if got > 0 {
			if err := s.store.SetCursor(ctx, s.cfg.Name, name, next); err != nil {
				return total, err
			}
			total += got
		}
		if got < s.cfg.MirrorBatch {
			return total, nil
		}
	}
}
