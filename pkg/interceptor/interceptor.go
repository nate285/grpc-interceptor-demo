package interceptor

import (
	"context"
	"fmt"
	"log"
	"reflect"
	//"runtime"
	"strconv"
	"math/rand"
	"time"

	"github.com/ori-edge/grpc-interceptor-demo/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// UnaryClientInterceptor is called on every request from a client to a unary
// server operation, here, we grab the operating system of the client and add it
// to the metadata within the context of the request so that it can be received
// by the server
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Get the operating system the client is running on
		rand.Seed(time.Now().UnixNano())

		tok := rand.Intn(10)

		tok_string := strconv.Itoa(tok)


		// Append the OS info to the outgoing request
		ctx = metadata.AppendToOutgoingContext(ctx, "tokens", tok_string)

		// Invoke the original method call
		err := invoker(ctx, method, req, reply, cc, opts...)

		log.Printf("client interceptor hit: appending OS: '%v' to metadata", tok_string)
		// Jiali: I suspect that before and after invoker are two time slot:
		// request and response, so before invoker we implement token and add it to metadata
		// while after invoker we handle the price on the response.
		return err
	}
}

// Embedded EdgeServerStream to allow us to access the RecvMsg function on
// intercept
type EdgeServerStream struct {
	grpc.ServerStream
}

// RecvMsg receives messages from a stream
func (e *EdgeServerStream) RecvMsg(m interface{}) error {
	// Here we can perform additional logic on the received message, such as
	// validation
	log.Printf("intercepted server stream message, type: %s", reflect.TypeOf(m).String())
	if err := e.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	return nil
}

// Set up a wrapper to allow us to access the RecvMsg function
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapper := &EdgeServerStream{
			ServerStream: ss,
		}
		return handler(srv, wrapper)
	}
}

// StreamClientInterceptor allows us to log on each client stream opening
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		log.Printf("opening client streaming to the server method: %v", method)

		return streamer(ctx, desc, cc, method)
	}
}

// UnaryServerInterceptor is called on every request received from a client to a
// unary server operation, here, we pull out the client operating system from
// the metadata, and inspect the context to receive the IP address that the
// request was received from. We then modify the EdgeLocation type to include
// this information for every request
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get the metadata from the incoming context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, fmt.Errorf("couldn't parse incoming context metadata")
		}

		// Retrieve the tokens
		tok := md.Get("tokens")
		// Get the client IP Address
		ip, err := getClientIP(ctx)
		if err != nil {
			return nil, err
		}

		// Populate the EdgeLocation type with the IP and tokens
		req.(*api.EdgeLocation).IpAddress = ip
		i, err := strconv.Atoi(tok[0])
		if(i < 5) {
			return nil, err
		}

		req.(*api.EdgeLocation).OperatingSystem = tok[0]

		h, err := handler(ctx, req)
		log.Printf("server interceptor hit: hydrating type with OS: '%v' and IP: '%v'", tok[0], ip)
		// Jiali: I suspect that before and after `handler` are two time slot:
		// request and response, so before it we implement overload handler
		// and check the metadata/token and do AQM.
		// while after `handler` we calculate the price update and add it to response.
		return h, err
	}
}

// GetClientIP inspects the context to retrieve the ip address of the client
func getClientIP(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("couldn't parse client IP address")
	}

	return p.Addr.String(), nil
}
