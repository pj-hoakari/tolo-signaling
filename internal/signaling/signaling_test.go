package signaling_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/pj-hoakari/tolo-signaling/internal/signaling"
	"github.com/pj-hoakari/tolo-signaling/internal/signaling/signalingmock"
)

func TestService_ListAliveEdges_usesAliveWindowCutoff(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	window := 30 * time.Second

	var gotCutoff time.Time
	repo.EXPECT().
		ListAliveEdges(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time) ([]signaling.Edge, error) {
			gotCutoff = cutoff
			return []signaling.Edge{}, nil
		})

	svc := signaling.NewService(repo,
		signaling.WithClock(func() time.Time { return now }),
		signaling.WithAliveWindow(window),
	)

	if _, err := svc.ListAliveEdges(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := now.Add(-window); !gotCutoff.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", gotCutoff, want)
	}
}

func TestService_RequestConnection(t *testing.T) {
	ctx := context.Background()

	t.Run("empty edge id is rejected without touching repo", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := signalingmock.NewMockRepository(ctrl)

		svc := signaling.NewService(repo)
		if _, err := svc.RequestConnection(ctx, ""); !errors.Is(err, signaling.ErrEdgeIDRequired) {
			t.Fatalf("err = %v, want ErrEdgeIDRequired", err)
		}
	})

	t.Run("missing presence is rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := signalingmock.NewMockRepository(ctrl)
		repo.EXPECT().EdgeExists(gomock.Any(), "edge-1").Return(false, nil)

		svc := signaling.NewService(repo)
		if _, err := svc.RequestConnection(ctx, "edge-1"); !errors.Is(err, signaling.ErrEdgeNotFound) {
			t.Fatalf("err = %v, want ErrEdgeNotFound", err)
		}
	})

	t.Run("creates session for existing edge", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := signalingmock.NewMockRepository(ctrl)
		gomock.InOrder(
			repo.EXPECT().EdgeExists(gomock.Any(), "edge-1").Return(true, nil),
			repo.EXPECT().CreateSession(gomock.Any(), "edge-1").Return("sess-123", nil),
		)

		svc := signaling.NewService(repo)
		got, err := svc.RequestConnection(ctx, "edge-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "sess-123" {
			t.Fatalf("sessionID = %q, want sess-123", got)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := signalingmock.NewMockRepository(ctrl)
		wantErr := errors.New("boom")
		repo.EXPECT().EdgeExists(gomock.Any(), "edge-1").Return(false, wantErr)

		svc := signaling.NewService(repo)
		if _, err := svc.RequestConnection(ctx, "edge-1"); !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	})
}
