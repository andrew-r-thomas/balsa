package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

type Balsa struct {
	pb.UnimplementedRaftServer
	grpcPort string
	siblings *[]sibling

	httpPort string
}

type sibling struct {
	addr   string
	client pb.RaftClient
}

func NewBalsa(grpcPort string, siblingPorts []string, httpPort string) Balsa {
	// make sibling connections
	log.Printf("%s: starting to make siblings\n", grpcPort)
	siblings := make([]sibling, len(siblingPorts))
	for i, sibPort := range siblingPorts {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(":"+sibPort, opts...)
		if err != nil {
			log.Fatalf("error creating client: %v\n", err)
		}
		client := pb.NewRaftClient(conn)
		siblings[i] = sibling{client: client, addr: sibPort}
	}

	return Balsa{grpcPort: grpcPort, siblings: &siblings, httpPort: httpPort}
}

func (balsa *Balsa) ServeGrpc() {
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRaftServer(grpcServer, balsa)

	listener, err := net.Listen("tcp", ":"+balsa.grpcPort)
	if err != nil {
		log.Fatalf("error starting lisener: %v\n", err)
	}
	log.Printf("lisenting on %s\n", balsa.grpcPort)
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("failed to serve grpc: %v\n", err)
	}
}

func (balsa *Balsa) Hello(ctx context.Context, request *pb.HelloRequest) (*pb.HelloResponse, error) {
	peer, _ := peer.FromContext(ctx)
	log.Printf("%s: got hello request from %s with name %s\n", balsa.grpcPort, peer.Addr, request.GetName())
	resp := pb.HelloResponse{
		Response: "Hello " + request.GetName(),
	}
	return &resp, nil
}

func (balsa *Balsa) ServeHttp() {
	// serve http
	// TODO: maybe change addr to node id or something for logging,
	// bc this isn't the address that will be serving the http
	log.Printf("setting up http server\n")
	http.HandleFunc("/say-hello", balsa.sayHello)
	log.Printf("http listening on %s\n", balsa.httpPort)
	http.ListenAndServe(":"+balsa.httpPort, nil)
}

func (balsa *Balsa) sayHello(w http.ResponseWriter, r *http.Request) {
	for _, sibling := range *balsa.siblings {
		log.Printf("%s: saying hello to %s\n", balsa.httpPort, sibling.addr)
		resp, err := sibling.client.Hello(context.Background(), &pb.HelloRequest{Name: "Sibling"})
		if err != nil {
			log.Printf("failed to say hello\n")
		}
		log.Printf("%s: got hello response from %s: %s\n", balsa.httpPort, sibling.addr, resp.GetResponse())
	}
}

type httpServer struct {
	addr     string
	siblings *[]sibling
}
