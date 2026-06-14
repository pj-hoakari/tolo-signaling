package signaling

import (
	"context"
	"errors"
	"time"
)

type Edge struct {
	ID         string
	LastSeenAt time.Time
}

type Repository interface {
	ListAliveEdges(ctx context.Context, cutoff time.Time) ([]Edge, error)
	EdgeExists(ctx context.Context, edgeID string) (bool, error)
	CreateSession(ctx context.Context, edgeID string) (sessionID string, err error)
}

var (
	ErrEdgeIDRequired = errors.New("signaling: edge id is required")
	ErrEdgeNotFound   = errors.New("signaling: edge not found")
)

const DefaultAliveWindow = 30 * time.Second

type Service struct {
	repo        Repository
	now         func() time.Time
	aliveWindow time.Duration
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

func WithAliveWindow(d time.Duration) Option {
	return func(s *Service) { s.aliveWindow = d }
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo:        repo,
		now:         time.Now,
		aliveWindow: DefaultAliveWindow,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) ListAliveEdges(ctx context.Context) ([]Edge, error) {
	cutoff := s.now().Add(-s.aliveWindow)
	return s.repo.ListAliveEdges(ctx, cutoff)
}

func (s *Service) RequestConnection(ctx context.Context, edgeID string) (string, error) {
	if edgeID == "" {
		return "", ErrEdgeIDRequired
	}
	exists, err := s.repo.EdgeExists(ctx, edgeID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrEdgeNotFound
	}
	return s.repo.CreateSession(ctx, edgeID)
}
