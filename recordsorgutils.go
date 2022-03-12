package main

import (
	pbrc "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
)

const (
	AVG_WIDTH = 3.3
)

func getFormatWidth(r *pbrc.Record) float32 {
	// Use the spine width if we have it
	if r.GetMetadata().GetRecordWidth() > 0 {
		// Make the adjustment for DS_F records
		if r.GetMetadata().GetSleeve() == pbrc.ReleaseMetadata_BAGS_UNLIMITED_PLAIN ||
			r.GetMetadata().GetSleeve() == pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP {
			return r.GetMetadata().GetRecordWidth() * 1.25
		}

		if r.GetMetadata().GetSleeve() == pbrc.ReleaseMetadata_SLEEVE_UNKNOWN {
			return r.GetMetadata().GetRecordWidth() * 1.15
		}

		if r.GetMetadata().GetSleeve() == pbrc.ReleaseMetadata_VINYL_STORAGE_NO_INNER {
			return r.GetMetadata().GetRecordWidth() * 1.4
		}

		return r.GetMetadata().GetRecordWidth()
	}

	return float32(AVG_WIDTH)
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
