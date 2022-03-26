package main

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rcpb "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
)

//ClientUpdate on an updated record
func (s *Server) ClientUpdate(ctx context.Context, req *rcpb.ClientUpdateRequest) (*rcpb.ClientUpdateResponse, error) {
	cache, err := s.loadCache(ctx)
	if err != nil {
		return nil, err
	}

	record, err := s.loadRecord(ctx, req.GetInstanceId())
	if err != nil {
		return nil, err
	}
	cache.Cache[req.GetInstanceId()] = s.buildCache(record)
	cache.Cache[req.GetInstanceId()].Width = s.getWidth(record)

	err = s.placeRecord(ctx, record, cache)
	if err != nil {
		return nil, err
	}

	return &rcpb.ClientUpdateResponse{}, s.saveCache(ctx, cache)
}

func (s *Server) GetOrg(ctx context.Context, req *pb.GetOrgRequest) (*pb.GetOrgResponse, error) {
	config, err := s.loadOrg(ctx)
	if err != nil {
		return nil, err
	}

	locations.Set(float64(len(config.GetOrgs())))

	for _, org := range config.GetOrgs() {
		if org.GetName() == req.GetOrgName() {
			return &pb.GetOrgResponse{Org: org}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "Unable to locate %v", req.GetOrgName())
}
