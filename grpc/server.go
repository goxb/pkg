package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ServerOption func(*serverOptions)

// ServerCredentialsConfig is credentials config
type ServerCredentialsConfig struct {
	CAPath   string
	CertPath string
	KeyPath  string
}

// ServerLogger set logger for grpc server
func ServerLogger(logger *zap.Logger) ServerOption {
	return func(o *serverOptions) {
		o.logger = logger
	}
}

// ServerCredentials set credentials
func ServerCredentials(creds ServerCredentialsConfig) ServerOption {
	return func(o *serverOptions) {
		o.creds = creds
	}
}

// UnaryServerInterceptors set unary server interceptors
func UnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServerOption {
	return func(o *serverOptions) {
		o.unaryInterceptors = interceptors
	}
}

// StreamServerInterceptors set stream server interceptors
func StreamServerInterceptors(interceptors ...grpc.StreamServerInterceptor) ServerOption {
	return func(o *serverOptions) {
		o.streamInterceptors = interceptors
	}
}

type serverOptions struct {
	logger             *zap.Logger
	creds              ServerCredentialsConfig
	streamInterceptors []grpc.StreamServerInterceptor
	unaryInterceptors  []grpc.UnaryServerInterceptor
}

func applyServerOptions(opts ...ServerOption) *serverOptions {
	o := &serverOptions{logger: zap.NewNop()}
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// NewGRPCServer create grpc server
func NewGRPCServer(options ...ServerOption) (*grpc.Server, error) {
	o := applyServerOptions(options...)

	opts := []grpc.ServerOption{}
	creds := o.creds
	if creds.CAPath == "" && creds.CertPath == "" && creds.KeyPath == "" {
		o.logger.Info("No TLS keys, insecure mode")
	} else {
		// TLS authentication, otherwise run without authentication.
		if creds.CAPath == "" {
			o.logger.Info("enable sample credentials in grpc server")
			tc, err := credentials.NewServerTLSFromFile(creds.CertPath, creds.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("bad tls credentials: %w", err)
			}

			opts = append(opts, grpc.Creds(tc))
		} else {
			o.logger.Info("enable mutual credentials in grpc server")
			// Parse certificates from client CA file to a new CertPool.
			cPool := x509.NewCertPool()
			if creds.CAPath != "" {
				caCert, err := ioutil.ReadFile(creds.CAPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read ac cert: %w", err)
				}
				if cPool.AppendCertsFromPEM(caCert) != true {
					return nil, fmt.Errorf("bad ac certificate: %w", err)
				}
			}

			// Parse certificates from client CA file to a new CertPool.
			certPair, err := tls.LoadX509KeyPair(creds.CertPath, creds.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("bad certificate of gRPC server, %w", err)
			}

			tlsConfig := tls.Config{
				Certificates: []tls.Certificate{certPair},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    cPool,
			}
			opts = append(opts, grpc.Creds(credentials.NewTLS(&tlsConfig)))
		}
	}

	// Add recovery middleware
	recoveryFn := func(err interface{}) error {
		o.logger.With(zap.Any("err", err)).Error("recover unkown execption")
		return nil
	}
	rec := grpc_recovery.WithRecoveryHandler(recoveryFn)

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_recovery.UnaryServerInterceptor(rec),
	}
	unaryInterceptors = append(unaryInterceptors, o.unaryInterceptors...)
	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_recovery.StreamServerInterceptor(rec),
	}
	streamInterceptors = append(streamInterceptors, o.streamInterceptors...)

	opts = append(opts,
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptors...)),
	)

	grpcServer := grpc.NewServer(opts...)
	return grpcServer, nil
}
