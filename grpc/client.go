package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ClientOption func(*clientOptions)

// ClientLogger set logger for grpc server
func ClientLogger(logger *zap.Logger) ClientOption {
	return func(o *clientOptions) {
		o.logger = logger
	}
}

// ClientRetryTimeout set timeout to reconnect
func ClientRetryTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.retryTimeout = timeout
	}
}

// ClientRetryMax set return max times to reconnect
func ClientRetryMax(max uint) ClientOption {
	return func(o *clientOptions) {
		o.retryMax = max
	}
}

// ClientCredentials set credentials
func ClientCredentials(creds ClientCredentialsConfig) ClientOption {
	return func(o *clientOptions) {
		o.creds = creds
	}
}

// ClientCInsecure set insecure to true
func ClientCInsecure() ClientOption {
	return func(o *clientOptions) {
		o.creds.Insecure = true
	}
}

// UnaryClientInterceptors set unary client interceptors
func UnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) ClientOption {
	return func(o *clientOptions) {
		o.unaryInterceptors = interceptors
	}
}

// StreamClientInterceptors set stream client interceptors
func StreamClientInterceptors(interceptors ...grpc.StreamClientInterceptor) ClientOption {
	return func(o *clientOptions) {
		o.streamInterceptors = interceptors
	}
}

// ClientCredentialsConfig is the config of client credentials
type ClientCredentialsConfig struct {
	CAPath     string
	CertPath   string
	KeyPath    string
	ServerName string
	Insecure   bool
}

// NewGRPCConn return a grpc connection from the config
func NewGRPCConn(address string, opts ...ClientOption) (*grpc.ClientConn, error) {
	o := applyClientOptions(opts...)

	var options []grpc.DialOption
	if o.creds.Insecure {
		options = append(options, grpc.WithInsecure())
	} else {
		clientTLSConfig := &tls.Config{
			ServerName: o.creds.ServerName,
		}

		if o.creds.CAPath != "" {
			cPool := x509.NewCertPool()
			caCert, err := ioutil.ReadFile(o.creds.CAPath)
			if err != nil {
				return nil, err
			}
			if cPool.AppendCertsFromPEM(caCert) != true {
				return nil, fmt.Errorf("failed to append ca crt: %w", err)
			}

			clientTLSConfig.RootCAs = cPool
		}

		if o.creds.CertPath != "" && o.creds.KeyPath != "" {
			clientCert, err := tls.LoadX509KeyPair(o.creds.CertPath, o.creds.KeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to append ca crt: %w", err)
			}
			clientTLSConfig.Certificates = []tls.Certificate{clientCert}
		}
		creds := credentials.NewTLS(clientTLSConfig)
		options = append(options, grpc.WithTransportCredentials(creds))
	}

	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(time.Second)),
		grpc_retry.WithMax(o.retryMax),
		grpc_retry.WithPerRetryTimeout(o.retryTimeout),
	}
	options = append(options,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(o.unaryInterceptors...)),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(o.streamInterceptors...)),
	)

	return grpc.Dial(address, options...)
}

type clientOptions struct {
	logger             *zap.Logger
	retryMax           uint
	retryTimeout       time.Duration
	creds              ClientCredentialsConfig
	streamInterceptors []grpc.StreamClientInterceptor
	unaryInterceptors  []grpc.UnaryClientInterceptor
}

func applyClientOptions(opts ...ClientOption) *clientOptions {
	o := &clientOptions{logger: zap.NewNop()}
	for _, opt := range opts {
		opt(o)
	}

	return o
}
