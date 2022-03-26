package main

import (
	"fmt"
	"sort"

	pbrc "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
	"golang.org/x/net/context"
)

const (
	AVG_WIDTH = 3.3
)

func (s *Server) getWidth(r *pbrc.Record) float32 {
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
		Ordering:    pb.Ordering_ORDERING_BY_LABEL,
		OrderString: "madeup",
	}
}

func (s *Server) placeRecord(ctx context.Context, record *pbrc.Record, cache *pb.OrderCache) error {
	orgs, err := s.loadOrg(ctx)
	if err != nil {
		return err
	}

	for _, org := range orgs.GetOrgs() {
		for _, place := range org.GetOrderings() {
			if place.GetInstanceId() == record.GetRelease().GetInstanceId() {
				// This record is placed
				nindex := s.getIndex(org, record, cache)

				if nindex == place.GetIndex() {
					//This record is in the right place
					s.Log(fmt.Sprintf("%v is in index %v", place.GetInstanceId(), nindex))
					return nil
				}

				s.removeRecord(org, record)
				return s.insertRecord(ctx, record, orgs, cache)
			}
		}
	}

	return s.insertRecord(ctx, record, orgs, cache)
}

func (s *Server) removeRecord(org *pb.Org, r *pbrc.Record) {
	index := int32(len(org.GetOrderings()))
	for _, entry := range org.GetOrderings() {
		if entry.GetInstanceId() == r.GetRelease().GetInstanceId() {
			index = entry.GetIndex()
		}
	}

	nord := make([]*pb.BuiltOrdering, 0)
	for _, entry := range org.GetOrderings() {
		if entry.GetInstanceId() != r.GetRelease().GetInstanceId() {
			if entry.GetIndex() > index {
				entry.Index--
			}
			nord = append(nord, entry)
		}
	}

	org.Orderings = nord
}

func (s *Server) insertRecord(ctx context.Context, record *pbrc.Record, orgs *pb.OrgConfig, cache *pb.OrderCache) error {
	// Record is not placed we need to run an insert
	for _, org := range orgs.GetOrgs() {
		for _, prop := range org.GetProperties() {
			if prop.GetFolderNumber() == record.GetRelease().GetFolderId() {
				rindex := s.getIndex(org, record, cache)
				slot := int32(1)

				for _, order := range org.GetOrderings() {
					if order.GetIndex() == rindex {
						slot = (order.GetSlotNumber())
					}
					if order.GetIndex() >= rindex {
						order.Index++
					}
				}

				org.Orderings = append(org.Orderings, &pb.BuiltOrdering{
					InstanceId: record.GetRelease().GetInstanceId(),
					SlotNumber: slot,
					Index:      rindex,
					FromFolder: prop.GetFolderNumber(),
				})

				s.validateWidths(org, cache)

				break
			}
		}
	}

	return s.saveOrg(ctx, orgs)
}

func (s *Server) validateWidths(o *pb.Org, cache *pb.OrderCache) {
	widths := make(map[int32]float32)
	for _, mwidth := range o.GetSlots() {
		widths[mwidth.GetSlotNumber()] = mwidth.GetSlotWidth()
	}

	for _, place := range o.GetOrderings() {
		width := cache.GetCache()[place.GetInstanceId()].GetWidth()
		widths[place.GetSlotNumber()] -= width
	}

	for slot, width := range widths {
		if width < 0 {
			s.RaiseIssue("Slot is too wide", fmt.Sprintf("Slot %v in %v is too wide: %v", slot, o.GetName(), width))
		}
	}
}

func (s *Server) getIndex(o *pb.Org, r *pbrc.Record, cache *pb.OrderCache) int32 {
	ordering := s.buildOrdering(o, cache)
	s.Log(fmt.Sprintf("Index of %v in %v", r.GetRelease().GetInstanceId(), ordering))
	for _, order := range ordering {
		if order.GetInstanceId() == r.GetRelease().GetInstanceId() {
			return order.GetIndex()
		}
	}

	return -1
}

func (s *Server) buildOrdering(o *pb.Org, cache *pb.OrderCache) []*pb.BuiltOrdering {
	instanceIds := make([]int32, 0)
	orderMap := make(map[int32]string)
	for _, elem := range o.GetOrderings() {
		instanceIds = append(instanceIds, elem.GetInstanceId())
		orderMap[elem.GetInstanceId()] = s.getOrderString(o, elem, cache)
	}

	sort.SliceStable(instanceIds, func(i, j int) bool {
		return orderMap[instanceIds[i]] < orderMap[instanceIds[j]]
	})

	ordering := make([]*pb.BuiltOrdering, 0)
	for i, iid := range instanceIds {
		ordering = append(ordering, &pb.BuiltOrdering{
			InstanceId: iid,
			Index:      int32(i) + 1,
		})
	}

	return ordering
}

func (s *Server) getOrderString(o *pb.Org, built *pb.BuiltOrdering, cache *pb.OrderCache) string {
	for _, props := range o.GetProperties() {
		if props.GetFolderNumber() == built.GetFromFolder() {
			for _, ordering := range cache.GetCache()[built.GetInstanceId()].GetOrderings() {
				if ordering.GetOrdering() == props.GetOrder() {
					return fmt.Sprintf("%v-%v", props.GetIndex(), ordering.GetOrderString())
				}
			}
		}
	}

	s.RaiseIssue("Ordering Cache Miss", fmt.Sprintf("%v has not ordering from %v and %v", built.GetInstanceId(), built, cache.GetCache()[built.GetInstanceId()]))
	return ""
}
