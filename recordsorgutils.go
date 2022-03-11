package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	pbrc "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
)

func (s *Server) runComputation(ctx context.Context) error {
	t := time.Now()
	sum := 0
	for i := 0; i < 10000; i++ {
		sum += i
	}
	s.Log(fmt.Sprintf("Sum is %v -> %v", sum, time.Now().Sub(t).Nanoseconds()/1000000))
	return nil
}

func (s *Server) buildCache(record *pbrc.Record) *pb.CacheStore {
	return &pb.CacheStore{Orderings: []*pb.CacheHolding{s.buildLabel(record)}}
}

func (s *Server) buildLabel(record *pbrc.Record) *pb.CacheHolding {
	return &pb.CacheHolding{
		Ordering:    pb.Ordering_BY_LABEL,
		OrderString: "madeup",
	}
}
