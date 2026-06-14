package firestore

import (
	"context"
	"errors"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pj-hoakari/tolo-signaling/internal/signaling"
)

const (
	edgesCollection        = "edges"
	sessionsSubcollection  = "sessions"
	sessionStatusRequested = "requested"
	lastSeenAtField        = "lastSeenAt"
)

type Store struct {
	client *fs.Client
}

var _ signaling.Repository = (*Store)(nil)

func New(ctx context.Context, projectID string, clientOptions ...option.ClientOption) (*Store, error) {
	client, err := fs.NewClient(ctx, projectID, clientOptions...)
	if err != nil {
		return nil, err
	}
	return &Store{client: client}, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

type sessionDoc struct {
	Status    string    `firestore:"status"`
	CreatedAt time.Time `firestore:"createdAt,serverTimestamp"`
}

func (s *Store) ListAliveEdges(ctx context.Context, cutoff time.Time) ([]signaling.Edge, error) {
	iter := s.client.Collection(edgesCollection).
		Where(lastSeenAtField, ">", cutoff).
		Documents(ctx)
	defer iter.Stop()

	edges := make([]signaling.Edge, 0)
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		var data struct {
			LastSeenAt time.Time `firestore:"lastSeenAt"`
		}
		if err := doc.DataTo(&data); err != nil {
			// Schema inconsistency: lastSeenAt がないドキュメントは無視
			continue
		}

		edges = append(edges, signaling.Edge{
			ID:         doc.Ref.ID,
			LastSeenAt: data.LastSeenAt,
		})
	}
	return edges, nil
}

func (s *Store) EdgeExists(ctx context.Context, edgeID string) (bool, error) {
	_, err := s.client.Collection(edgesCollection).Doc(edgeID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) CreateSession(ctx context.Context, edgeID string) (string, error) {
	sessionRef := s.client.Collection(edgesCollection).Doc(edgeID).
		Collection(sessionsSubcollection).NewDoc()
	if _, err := sessionRef.Set(ctx, sessionDoc{Status: sessionStatusRequested}); err != nil {
		return "", err
	}
	return sessionRef.ID, nil
}
