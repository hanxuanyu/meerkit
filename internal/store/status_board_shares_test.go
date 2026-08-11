package store

import (
	"context"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestStatusBoardShareLifecycle(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	share := core.StatusBoardShare{ID: "share-1", Token: "public-token", Name: "Public status", MonitorIDs: []string{"monitor-1"}, ItemIDs: []string{"item-1"}, Active: true, CreatedAt: time.Now().UTC()}
	if err := database.CreateStatusBoardShare(ctx, share); err != nil {
		t.Fatal(err)
	}
	shares, err := database.ListStatusBoardShares(ctx)
	if err != nil || len(shares) != 1 || shares[0].Name != share.Name {
		t.Fatalf("shares=%+v err=%v", shares, err)
	}
	loaded, err := database.GetStatusBoardShareByToken(ctx, share.Token)
	if err != nil || loaded.ID != share.ID || !loaded.Active || len(loaded.MonitorIDs) != 1 || len(loaded.ItemIDs) != 1 {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	if err := database.SetStatusBoardShareActive(ctx, share.ID, false); err != nil {
		t.Fatal(err)
	}
	loaded, err = database.GetStatusBoardShareByToken(ctx, share.Token)
	if err != nil || loaded.Active {
		t.Fatalf("deactivated share state=%+v err=%v", loaded, err)
	}
	if err := database.SetStatusBoardShareActive(ctx, share.ID, true); err != nil {
		t.Fatal(err)
	}
	loaded, err = database.GetStatusBoardShareByToken(ctx, share.Token)
	if err != nil || !loaded.Active {
		t.Fatalf("restored share state=%+v err=%v", loaded, err)
	}
	if deleted, err := database.DeleteStatusBoardShare(ctx, share.ID); err != nil || deleted {
		t.Fatalf("active share was permanently deleted: deleted=%v err=%v", deleted, err)
	}
	if err := database.SetStatusBoardShareActive(ctx, share.ID, false); err != nil {
		t.Fatal(err)
	}
	if deleted, err := database.DeleteStatusBoardShare(ctx, share.ID); err != nil || !deleted {
		t.Fatalf("inactive share was not permanently deleted: deleted=%v err=%v", deleted, err)
	}
	if _, err := database.GetStatusBoardShareByToken(ctx, share.Token); !IsNoRows(err) {
		t.Fatalf("permanently deleted share remained accessible: %v", err)
	}
}
