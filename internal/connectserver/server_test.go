package connectserver_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/mock/gomock"

	signalingv1 "github.com/pj-hoakari/tolo-signaling/gen/signaling/v1"
	"github.com/pj-hoakari/tolo-signaling/gen/signaling/v1/signalingv1connect"
	"github.com/pj-hoakari/tolo-signaling/internal/connectserver"
	"github.com/pj-hoakari/tolo-signaling/internal/signaling"
	"github.com/pj-hoakari/tolo-signaling/internal/signaling/signalingmock"
)

func newTestServer(t *testing.T, repo signaling.Repository, opts ...signaling.Option) (
	signalingv1connect.EdgeRegistryServiceClient,
	signalingv1connect.SignalingServiceClient,
) {
	t.Helper()

	svc := signaling.NewService(repo, opts...)
	srv := httptest.NewServer(connectserver.NewHandler(svc))
	t.Cleanup(srv.Close)

	httpClient := srv.Client()
	return signalingv1connect.NewEdgeRegistryServiceClient(httpClient, srv.URL),
		signalingv1connect.NewSignalingServiceClient(httpClient, srv.URL)
}

func TestListAliveEdges_returnsEdges(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	seen := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	repo.EXPECT().
		ListAliveEdges(gomock.Any(), gomock.Any()).
		Return([]signaling.Edge{{ID: "edge-1", LastSeenAt: seen}}, nil)

	edgeClient, _ := newTestServer(t, repo)

	resp, err := edgeClient.ListAliveEdges(context.Background(),
		connect.NewRequest(&signalingv1.ListAliveEdgesRequest{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Msg.Edges) != 1 {
		t.Fatalf("edges len = %d, want 1", len(resp.Msg.Edges))
	}
	got := resp.Msg.Edges[0]
	if got.Id != "edge-1" {
		t.Errorf("id = %q, want edge-1", got.Id)
	}
	if !got.LastSeenAt.AsTime().Equal(seen) {
		t.Errorf("lastSeenAt = %v, want %v", got.LastSeenAt.AsTime(), seen)
	}
}

func TestListAliveEdges_passesAliveWindowCutoff(t *testing.T) {
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

	edgeClient, _ := newTestServer(t, repo,
		signaling.WithClock(func() time.Time { return now }),
		signaling.WithAliveWindow(window),
	)

	if _, err := edgeClient.ListAliveEdges(context.Background(),
		connect.NewRequest(&signalingv1.ListAliveEdgesRequest{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := now.Add(-window); !gotCutoff.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", gotCutoff, want)
	}
}

func TestListAliveEdges_repoErrorMapsToInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	repo.EXPECT().
		ListAliveEdges(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("firestore unavailable"))

	edgeClient, _ := newTestServer(t, repo)

	_, err := edgeClient.ListAliveEdges(context.Background(),
		connect.NewRequest(&signalingv1.ListAliveEdgesRequest{}))
	if got := connect.CodeOf(err); got != connect.CodeInternal {
		t.Fatalf("code = %v, want Internal (err=%v)", got, err)
	}
}

func TestRequestConnection_createsSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	gomock.InOrder(
		repo.EXPECT().EdgeExists(gomock.Any(), "edge-1").Return(true, nil),
		repo.EXPECT().CreateSession(gomock.Any(), "edge-1").Return("sess-1", nil),
	)

	_, sigClient := newTestServer(t, repo)

	resp, err := sigClient.RequestConnection(context.Background(),
		connect.NewRequest(&signalingv1.RequestConnectionRequest{EdgeId: "edge-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Msg.SessionId != "sess-1" {
		t.Fatalf("sessionId = %q, want sess-1", resp.Msg.SessionId)
	}
}

func TestRequestConnection_edgeNotFoundMapsToNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	repo.EXPECT().EdgeExists(gomock.Any(), "missing").Return(false, nil)

	_, sigClient := newTestServer(t, repo)

	_, err := sigClient.RequestConnection(context.Background(),
		connect.NewRequest(&signalingv1.RequestConnectionRequest{EdgeId: "missing"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("code = %v, want NotFound (err=%v)", got, err)
	}
}

func TestRequestConnection_emptyEdgeIDMapsToInvalidArgument(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := signalingmock.NewMockRepository(ctrl)

	_, sigClient := newTestServer(t, repo)

	_, err := sigClient.RequestConnection(context.Background(),
		connect.NewRequest(&signalingv1.RequestConnectionRequest{EdgeId: ""}))
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument (err=%v)", got, err)
	}
}
