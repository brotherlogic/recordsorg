package main

import (
	rcpb "github.com/brotherlogic/recordcollection/proto"
	"golang.org/x/net/context"
)

//ClientUpdate on an updated record
func (s *Server) ClientUpdate(ctx context.Context, req *rcpb.ClientUpdateRequest) (*rcpb.ClientUpdateResponse, error) {
	cache, err := s.loadCache(ctx)
	if err != nil {
		return &rcpb.ClientUpdateResponse{}, err
	}

	record, err := s.loadRecord(ctx, req.GetInstanceId())
	cache.Cache[req.GetInstanceId()] = s.buildCache(record)
	cache.Cache[req.GetInstanceId()].Width = s.getWidth(record)

	return &rcpb.ClientUpdateResponse{}, s.saveCache(ctx, cache)
}
