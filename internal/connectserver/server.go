package connectserver

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"

	signalingv1 "github.com/pj-hoakari/tolo-signaling/gen/signaling/v1"
	"github.com/pj-hoakari/tolo-signaling/gen/signaling/v1/signalingv1connect"
	"github.com/pj-hoakari/tolo-signaling/internal/signaling"
)

type handler struct {
	signalingv1connect.UnimplementedEdgeRegistryServiceHandler
	signalingv1connect.UnimplementedSignalingServiceHandler
	svc *signaling.Service
}

func (h *handler) ListAliveEdges(
	ctx context.Context,
	_ *connect.Request[signalingv1.ListAliveEdgesRequest],
) (*connect.Response[signalingv1.ListAliveEdgesResponse], error) {
	edges, err := h.svc.ListAliveEdges(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}

	pb := make([]*signalingv1.Edge, 0, len(edges))
	for _, e := range edges {
		pb = append(pb, &signalingv1.Edge{
			Id:         e.ID,
			LastSeenAt: timestamppb.New(e.LastSeenAt),
		})
	}
	return connect.NewResponse(&signalingv1.ListAliveEdgesResponse{Edges: pb}), nil
}

func (h *handler) RequestConnection(
	ctx context.Context,
	req *connect.Request[signalingv1.RequestConnectionRequest],
) (*connect.Response[signalingv1.RequestConnectionResponse], error) {
	sessionID, err := h.svc.RequestConnection(ctx, req.Msg.GetEdgeId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&signalingv1.RequestConnectionResponse{SessionId: sessionID}), nil
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, signaling.ErrEdgeIDRequired):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, signaling.ErrEdgeNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

// TODO: 認証機構繋ぎ込み
func NewHandler(svc *signaling.Service, opts ...connect.HandlerOption) http.Handler {
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.Handle(signalingv1connect.NewEdgeRegistryServiceHandler(h, opts...))
	mux.Handle(signalingv1connect.NewSignalingServiceHandler(h, opts...))
	return mux
}

func NewH2CHandler(h http.Handler) http.Handler {
	return h2c.NewHandler(h, &http2.Server{})
}
