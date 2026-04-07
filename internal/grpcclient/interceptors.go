package grpcclient

import (
	"context"

	"github.com/agynio/media-proxy/internal/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func identityUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = appendIdentityMetadata(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func identityStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = appendIdentityMetadata(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func appendIdentityMetadata(ctx context.Context) context.Context {
	resolved, ok := identity.IdentityFromContext(ctx)
	if !ok {
		// No identity in context; keep metadata unchanged.
		return ctx
	}

	return metadata.AppendToOutgoingContext(
		ctx,
		identity.MetadataKeyIdentityID,
		resolved.IdentityID,
		identity.MetadataKeyIdentityType,
		string(resolved.IdentityType),
	)
}
