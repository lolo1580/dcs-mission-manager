package app

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/syncer"
)

// syncMu serialises automatic uploads. A second trigger (startup, a debrief
// arriving while a sync is running) is skipped rather than queued: the next
// debrief or the manual button will send the new data anyway.
var syncMu sync.Mutex

// trySync uploads the recent live missions when the desktop is configured. It
// is best-effort and never blocks DCS ingestion or the UI.
func trySync(database *db.DB) {
	if database == nil {
		return
	}
	root, err := config.UserDataRoot()
	if err != nil {
		log.Printf("sync: data folder: %v", err)
		return
	}
	user, settings, err := syncer.EnsureSettings(root)
	if err != nil {
		log.Printf("sync: settings: %v", err)
		return
	}
	if user.ServerURL == "" || user.SyncToken == "" {
		return
	}
	if !syncMu.TryLock() {
		return
	}
	defer syncMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	started := time.Now().UnixMilli()
	client := syncer.NewClient(user.ServerURL, user.SyncToken, settings.ClientID, database)
	result, uploadErr := client.Upload(ctx)
	documents, cursor, documentErr := client.SyncDocuments(ctx, database, settings.SyncCursor)
	if documentErr == nil {
		_ = syncer.SaveCursor(root, cursor)
	}
	if uploadErr != nil && documentErr != nil {
		log.Printf("sync: missions: %v; documents: %v", uploadErr, documentErr)
		_ = syncer.RecordError(root, "error: "+uploadErr.Error())
		return
	}
	if uploadErr != nil {
		log.Printf("sync: missions: %v", uploadErr)
	}
	if documentErr != nil {
		log.Printf("sync: documents: %v", documentErr)
	}
	detail := fmt.Sprintf("%d mission(s), %d plan(s), %d checklist(s)",
		result.Imported, documents.Plans, documents.Checklists)
	if uploadErr != nil {
		detail = "missions error; " + detail
	}
	if documentErr != nil {
		detail = "documents error; " + detail
	}
	if documents.Conflicts > 0 {
		detail += fmt.Sprintf(", %d conflict(s)", documents.Conflicts)
	}
	if documentErr != nil {
		_ = syncer.RecordError(root, detail)
	} else {
		_ = syncer.RecordSync(root, result.Imported, detail, started)
	}
	if result.Imported > 0 || documents.Plans > 0 || documents.Checklists > 0 {
		log.Printf("sync: uploaded %d mission(s), %d plan(s), %d checklist(s) to %s",
			result.Imported, documents.Plans, documents.Checklists, user.ServerURL)
	}
}
