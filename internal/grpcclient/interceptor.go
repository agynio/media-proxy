package grpcclient

import (
	"context"

	"github.com/agynio/media-proxy/internal/identity"
	"google.golang.org/grpc"
)

func IdentityUnaryInterceptor(
	ctx context.Context,
	method string,
	req any,
	reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx = identity.AppendToOutgoingContext(ctx)
	return invoker(ctx, method, req, reply, cc, opts...)
}

func IdentityStreamInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	ctx = identity.AppendToOutgoingContext(ctx)
	return streamer(ctx, desc, cc, method, opts...)
}
