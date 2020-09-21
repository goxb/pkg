package grpc

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoggerFeild func(context.Context) zap.Field

func applyLoggerFeilds(ctx context.Context, lfs ...LoggerFeild) []zap.Field {
	var fields []zap.Field
	for _, ls := range lfs {
		fields = append(fields, ls(ctx))
	}

	return fields
}

// UnaryServerInterceptor returns a new unary server interceptors that adds zap.Logger to the context.
func UnaryServerInterceptor(logger *zap.Logger, opts ...LoggerFeild) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)
		endTime := time.Now()

		code := status.Code(err)
		fields := applyLoggerFeilds(ctx, opts...)

		var msg string
		if err != nil {
			msg = fmt.Sprintf("latency=%-12s %s %s, err=%s", endTime.Sub(startTime).String(), info.FullMethod, code.String(), err)
		} else {
			msg = fmt.Sprintf("latency=%-12s %s %s", endTime.Sub(startTime).String(), info.FullMethod, code.String())
		}

		if code == codes.Internal || code == codes.Unavailable {
			logger.With(fields...).Error(msg)
		} else {
			logger.With(fields...).Info(msg)
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a new streaming server interceptor that adds zap.Logger to the context.
func StreamServerInterceptor(logger *zap.Logger, opts ...LoggerFeild) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()

		startTime := time.Now()
		err := handler(srv, stream)
		endTime := time.Now()

		code := status.Code(err)
		fields := applyLoggerFeilds(ctx, opts...)
		var msg string
		if err != nil {
			msg = fmt.Sprintf("latency=%-12s %s %s, err=%s", endTime.Sub(startTime).String(), info.FullMethod, code.String(), err)
		} else {
			msg = fmt.Sprintf("latency=%-12s %s %s", endTime.Sub(startTime).String(), info.FullMethod, code.String())
		}

		if code == codes.Internal || code == codes.Unavailable {
			logger.With(fields...).Error(msg)
		} else {
			logger.With(fields...).Info(msg)
		}

		return err
	}
}
