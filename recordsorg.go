package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	dspb "github.com/brotherlogic/dstore/proto"
	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
	rcpb "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"

	google_protobuf "github.com/golang/protobuf/ptypes/any"
)

const (
	CACHE_KEY = "github.com/brotherlogic/recordsorg/cache"
	ORG_KEY   = "github.com/brotherlogic/recordsorg/org"
)

var (
	//Backlog - the print queue
	count = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordsorg_cache_count",
		Help: "The size of the tracking queue",
	})
	size = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordsorg_cache_size",
		Help: "The size of the tracking queue",
	})
)

//Server main server type
type Server struct {
	*goserver.GoServer
}

// Init builds the server
func Init() *Server {
	s := &Server{
		GoServer: &goserver.GoServer{},
	}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {
	rcpb.RegisterClientUpdateServiceServer(server, s)
}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{
		{Key: "magic", Value: int64(12345)},
	}
}

func (s *Server) load(ctx context.Context, key string) ([]byte, error) {
	conn, err := s.FDialServer(ctx, "dstore")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := dspb.NewDStoreServiceClient(conn)
	res, err := client.Read(ctx, &dspb.ReadRequest{Key: CACHE_KEY})
	if err != nil {
		return nil, err
	}

	if res.GetConsensus() < 0.5 {
		return nil, fmt.Errorf("could not get read consensus (%v)", res.GetConsensus())
	}

	return res.GetValue().GetValue(), nil
}

func (s *Server) loadCache(ctx context.Context) (*pb.OrderCache, error) {
	data, err := s.load(ctx, CACHE_KEY)
	if err != nil {
		if status.Convert(err).Code() == codes.InvalidArgument {
			return &pb.OrderCache{Cache: make(map[int32]*pb.CacheStore)}, nil
		}
		return nil, err
	}

	cache := &pb.OrderCache{}
	err = proto.Unmarshal(data, cache)
	if err != nil {
		return nil, err
	}

	count.Set(float64(len(cache.GetCache())))
	size.Set(float64(proto.Size(cache)))

	return cache, nil
}

func (s *Server) loadOrg(ctx context.Context) (*pb.OrgConfig, error) {
	data, err := s.load(ctx, ORG_KEY)
	if err != nil {
		if status.Convert(err).Code() == codes.InvalidArgument {
			config := &pb.OrgConfig{}
			org := &pb.Org{
				Name: "12 Inches",
				Properties: []*pb.FolderProperties{
					{
						FolderNumber: 3903712,
						Index:        1,
						Order:        pb.Ordering_ORDERING_BY_LABEL,
						PreSpace:     false,
					},
					{
						FolderNumber: 242017,
						Index:        2,
						Order:        pb.Ordering_ORDERING_BY_LABEL,
						PreSpace:     false,
					},
					{
						FolderNumber: 3282985,
						Index:        3,
						Order:        pb.Ordering_ORDERING_BY_DATE_ADDED,
						PreSpace:     true,
					}},
			}
			config.Orgs = []*pb.Org{org}
			return config, nil
		}
		return nil, err
	}

	config := &pb.OrgConfig{}
	err = proto.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (s *Server) saveCache(ctx context.Context, config *pb.OrderCache) error {
	data, err := proto.Marshal(config)
	if err != nil {
		return err
	}
	return s.save(ctx, data, CACHE_KEY)
}

func (s *Server) saveOrg(ctx context.Context, config *pb.OrgConfig) error {
	data, err := proto.Marshal(config)
	if err != nil {
		return err
	}
	return s.save(ctx, data, ORG_KEY)
}

func (s *Server) save(ctx context.Context, data []byte, key string) error {
	conn, err := s.FDialServer(ctx, "dstore")
	if err != nil {
		return err
	}
	defer conn.Close()

	client := dspb.NewDStoreServiceClient(conn)
	res, err := client.Write(ctx, &dspb.WriteRequest{Key: key, Value: &google_protobuf.Any{Value: data}})
	if err != nil {
		return err
	}

	if res.GetConsensus() < 0.5 {
		return fmt.Errorf("could not get write consensus (%v)", res.GetConsensus())
	}

	return nil
}

func (s *Server) loadRecord(ctx context.Context, id int32) (*rcpb.Record, error) {
	conn, err := s.FDialServer(ctx, "recordcollection")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := rcpb.NewRecordCollectionServiceClient(conn)

	resp, err := client.GetRecord(ctx, &rcpb.GetRecordRequest{InstanceId: id})
	if err != nil {
		return nil, err
	}
	return resp.GetRecord(), nil
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.PrepServer()
	server.Register = server

	err := server.RegisterServerV2("recordsorg", false, true)
	if err != nil {
		return
	}

	// Preload metrics
	ctx, cancel := utils.ManualContext("recordsorg-init", time.Minute)
	_, err = server.loadCache(ctx)
	if err != nil {
		cancel()
		log.Fatalf("Unable to load initial cache: %v", err)
	}
	cancel()

	fmt.Printf("%v", server.Serve())
}
