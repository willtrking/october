package october

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type grpcServerRegistrar func(*grpc.Server) // Interface for generated GRPC server registrars

type GRPCServer struct {
	Server        *grpc.Server

	mode Mode

	address string
	port    int

	tlsCrt string
	tlsKey string

	tlsOpt grpc.ServerOption

	externalUnaryInterceptors  []grpc.UnaryServerInterceptor
	externalStreamInterceptors []grpc.StreamServerInterceptor

	additionalOpts []grpc.ServerOption
}

func (g *GRPCServer) Name() string {
	return "grpc"
}

func (g *GRPCServer) Address() string {
	return fmt.Sprintf("%s:%d", g.address, g.port)
}

func (g *GRPCServer) WithTLS(crt, key string) error {
	zap.L().Named("OCTOBER").Info("Reconfiguring controlled GRPC server with TLS")

	crt = strings.TrimSpace(crt)
	key = strings.TrimSpace(key)

	if crt == "" && key == "" {
		zap.L().Named("OCTOBER").Info("Controlled GRPC server configured without TLS")

	} else if crt != "" && key != "" {

		zap.L().Named("OCTOBER").Info("Controlled GRPC server received TLS credential paths")

	} else if crt != "" && key == "" {

		zap.L().Named("OCTOBER").Warn("Controlled GRPC server received TLS CRT bundle WITHOUT key")

	} else if crt == "" && key != "" {

		zap.L().Named("OCTOBER").Warn("Controlled GRPC server received TLS key WITHOUT CRT bundle")
	}

	grpcTls, grpcTlsErr := GRPCTLSCreds(crt, key)
	if grpcTlsErr != nil {
		return grpcTlsErr
	}

	g.tlsCrt = crt
	g.tlsKey = key
	g.tlsOpt = grpcTls

	g.rebuildServer()

	return nil
}

func (g *GRPCServer) MustWithTLS(crt, key string) {

	err := g.WithTLS(crt, key)

	if err != nil {
		zap.L().Named("OCTOBER").Fatal("Failed to generate pre-configured GRPC Server", zap.Error(err))
	}
}

func (g *GRPCServer) WithInterceptors(unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor) error {
	zap.S().Named("OCTOBER").Infof("Reconfiguring controlled GRPC server with interceptors (%d unary, %d stream)", len(unary), len(stream))

	g.externalUnaryInterceptors = unary
	g.externalStreamInterceptors = stream

	g.rebuildServer()

	return nil
}

func (g *GRPCServer) WithServerOptions(opt ...grpc.ServerOption) {
	g.additionalOpts = opt

	g.rebuildServer()
}

func (g *GRPCServer) rebuildServer() {

	unaryInterceptors, streamInterceptors := GRPCServerInstrumentation(g.mode)


	unaryInterceptors = append(unaryInterceptors, g.externalUnaryInterceptors...)
	streamInterceptors = append(streamInterceptors, g.externalStreamInterceptors...)

	var tlsOpt grpc.ServerOption

	if g.tlsOpt == nil {
		tlsOpt = grpc.Creds(nil)
	} else {
		tlsOpt = g.tlsOpt
	}

	var allOpts []grpc.ServerOption
	allOpts = append(allOpts, tlsOpt)
	allOpts = append(allOpts, grpc_middleware.WithUnaryServerChain(unaryInterceptors...), grpc_middleware.WithStreamServerChain(streamInterceptors...))
	allOpts = append(allOpts, g.additionalOpts...)

	g.Server = grpc.NewServer(allOpts...)

}

func (g *GRPCServer) MustWithInterceptors(unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor) {

	err := g.WithInterceptors(unary, stream)

	if err != nil {
		zap.L().Named("OCTOBER").Fatal("Failed to generate pre-configured GRPC Server", zap.Error(err))
	}
}

func (g *GRPCServer) WithServiceRegistrars(registrars ...grpcServerRegistrar) {

	for _, registrar := range registrars {
		registrar(g.Server)
	}
}

func (g *GRPCServer) Start() (bool, error) {

	address := g.Address()

	zap.S().Named("OCTOBER").Infof("Starting controlled GRPC server (%s)...", address)

	lis, err := net.Listen("tcp", address)

	if lis != nil {
		defer lis.Close()
	}

	if err != nil {
		return false, err
	}

	err = g.Server.Serve(lis)

	return err == nil, err

}

func (g *GRPCServer) Shutdown(ctx context.Context) error {
	address := g.Address()
	zap.S().Named("OCTOBER").Infof("Gracefully stopping controlled GRPC (%s)...", address)

	g.Server.GracefulStop()

	return nil
}


func serverPayloadDecider(m Mode) grpc_logging.ServerPayloadLoggingDecider {

	if m == LOCAL {
		return func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
			return true
		}
	}

	return nil
}

func grpcZapOptions(mode Mode) []grpc_zap.Option {
	return []grpc_zap.Option{
		grpc_zap.WithLevels(grpc_zap.DefaultCodeToLevel),
		grpc_zap.WithDurationField(func(duration time.Duration) zapcore.Field {
			return zap.Float64("grpc.time_ms", float64(duration.Nanoseconds())/float64(time.Millisecond))
		}),
	}
}

func GRPCTLSCreds(crt, key string) (grpc.ServerOption, error) {
	if crt != "" && key != "" {
		peerCert, peerCertErr := tls.LoadX509KeyPair(crt, key)
		if peerCertErr != nil {
			return nil, peerCertErr
		}

		ta := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{peerCert},
			//ClientCAs:    caCertPool,
			//ClientAuth:   tls.RequireAndVerifyClientCert,
		})

		return grpc.Creds(ta), nil
	}

	return grpc.Creds(nil), nil

}

func GRPCServerInstrumentation(mode Mode) ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {

	loggingOpts := grpcZapOptions(mode)

	var unary []grpc.UnaryServerInterceptor
	var stream []grpc.StreamServerInterceptor

	/*if mode == LOCAL || mode == DEV {
		grpc_prometheus.EnableHandlingTimeHistogram()
	}*/

	unary = append(unary, grpc_zap.UnaryServerInterceptor(zap.L(), loggingOpts...))
	stream = append(stream, grpc_zap.StreamServerInterceptor(zap.L(), loggingOpts...))

	payloadDecider := serverPayloadDecider(mode)

	if payloadDecider != nil {
		unary = append(unary, grpc_zap.PayloadUnaryServerInterceptor(zap.L(), payloadDecider))
		stream = append(stream, grpc_zap.PayloadStreamServerInterceptor(zap.L(), payloadDecider))
	}

	return unary, stream
}
