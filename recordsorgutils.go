package main

import (
	"fmt"
	"sort"

	"github.com/brotherlogic/goserver/utils"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
	"golang.org/x/net/context"

	gd "github.com/brotherlogic/godiscogs"
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
	return &pb.CacheStore{Orderings: []*pb.CacheHolding{
		s.buildLabel(record),
		s.buildIID(record),
	}}
}

func (s *Server) buildLabel(record *pbrc.Record) *pb.CacheHolding {
	label := gd.GetMainLabel(record.GetRelease().GetLabels())
	return &pb.CacheHolding{
		Ordering:    pb.Ordering_ORDERING_BY_LABEL,
		OrderString: label.GetName() + "-" + label.GetCatno(),
	}
}

func (s *Server) buildIID(record *pbrc.Record) *pb.CacheHolding {
	return &pb.CacheHolding{
		Ordering:    pb.Ordering_ORDERING_BY_DATE_ADDED,
		OrderString: fmt.Sprintf("%v", record.GetMetadata().GetDateAdded()),
	}
}

func (s *Server) placeRecord(ctx context.Context, record *pbrc.Record, cache *pb.OrderCache) error {
	orgs, err := s.loadOrg(ctx)
	if err != nil {
		return err
	}
	return s.placeRecordIntoOrgs(ctx, record, cache, orgs)
}

func (s *Server) placeRecordIntoOrgs(ctx context.Context, record *pbrc.Record, cache *pb.OrderCache, orgs *pb.OrgConfig) error {
	for _, org := range orgs.GetOrgs() {
		for _, place := range org.GetOrderings() {
			if place.GetInstanceId() == record.GetRelease().GetInstanceId() {
				s.removeRecord(ctx, org, record)
			}
		}
	}

	for _, org := range orgs.GetOrgs() {
		for _, place := range org.Properties {
			if place.FolderNumber == record.GetRelease().GetFolderId() {
				s.insertRecord(ctx, record, org, cache)
			}
		}
	}

	return s.saveOrg(ctx, orgs)
}

func (s *Server) removeRecord(ctx context.Context, org *pb.Org, r *pbrc.Record) {
	s.Log(fmt.Sprintf("Removing %v from %v", r.GetRelease().GetInstanceId(), org.GetName()))
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

func (s *Server) insertRecord(ctx context.Context, record *pbrc.Record, org *pb.Org, cache *pb.OrderCache) error {
	s.Log(fmt.Sprintf("Adding %v into %v", record.GetRelease().GetInstanceId(), org.GetName()))
	// Record is not placed we need to run an insert
	for _, prop := range org.GetProperties() {
		if prop.GetFolderNumber() == record.GetRelease().GetFolderId() {
			rindex, orderstr := s.getIndex(ctx, org, record, cache)
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
				TakenWidth: cache.GetCache()[record.GetRelease().GetInstanceId()].GetWidth(),
				Ordered:    orderstr,
			})

			s.validateWidths(org, cache)

			break
		}
	}

	return nil
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

func (s *Server) getIndex(ctx context.Context, o *pb.Org, r *pbrc.Record, cache *pb.OrderCache) (int32, string) {
	orderMap := make(map[int32]string)
	for _, order := range o.GetOrderings() {
		orderMap[order.GetInstanceId()] = s.getOrderString(ctx, o, order, cache)
	}

	sort.SliceStable(o.Orderings, func(i, j int) bool {
		return orderMap[o.Orderings[i].InstanceId] < orderMap[o.Orderings[j].InstanceId]
	})

	oString := ""
	for _, props := range o.GetProperties() {
		if props.GetFolderNumber() == r.GetRelease().GetFolderId() {
			for _, or := range cache.GetCache()[r.GetRelease().GetInstanceId()].GetOrderings() {
				if or.GetOrdering() == props.GetOrder() {
					oString = fmt.Sprintf("%v-%v", props.GetIndex(), or.GetOrderString())
				}
			}
		}
	}

	s.Log(fmt.Sprintf("Placing %v with %v", r.GetRelease().GetInstanceId(), oString))
	for _, val := range o.Orderings {
		if oString < s.getOrderString(ctx, o, val, cache) {
			s.Log(fmt.Sprintf("Found higher: %v", s.getOrderString(ctx, o, val, cache)))
			return val.GetIndex(), oString
		}
	}

	return 0, oString
}

func (s *Server) buildOrdering(ctx context.Context, o *pb.Org, cache *pb.OrderCache) []*pb.BuiltOrdering {
	instanceIds := make([]int32, 0)
	orderMap := make(map[int32]string)
	fMap := make(map[int32]*pb.BuiltOrdering)
	for _, elem := range o.GetOrderings() {
		instanceIds = append(instanceIds, elem.GetInstanceId())
		orderMap[elem.GetInstanceId()] = s.getOrderString(ctx, o, elem, cache)
		fMap[elem.GetInstanceId()] = elem
	}

	sort.SliceStable(instanceIds, func(i, j int) bool {
		return orderMap[instanceIds[i]] < orderMap[instanceIds[j]]
	})

	ordering := make([]*pb.BuiltOrdering, 0)
	for i, iid := range instanceIds {
		fMap[iid].Index = int32(i) + 1
		fMap[iid].TakenWidth = cache.GetCache()[iid].GetWidth()
		fMap[iid].Ordered = orderMap[iid]
		ordering = append(ordering, fMap[iid])
	}

	return ordering
}

// Places ordered items into the correct slots
func (s *Server) slotify(ctx context.Context, o *pb.Org, ordering []*pb.BuiltOrdering, cache *pb.OrderCache) {
	currSlot := int32(1)
	sWidth := float32(0)
	cFolder := int32(0)

	for _, bo := range ordering {
		if bo.FromFolder != cFolder {
			cFolder = bo.FromFolder
			for _, og := range o.GetProperties() {
				if og.FolderNumber == cFolder && og.PreSpace {
					currSlot++
				}
			}
		}

		sWidth += cache.GetCache()[bo.GetInstanceId()].GetWidth()
		for _, w := range o.GetSlots() {
			if w.GetSlotNumber() == currSlot && sWidth > w.GetSlotWidth() {
				currSlot++
				sWidth = 0
			}
		}
	}

}

func (s *Server) getOrderString(ctx context.Context, o *pb.Org, built *pb.BuiltOrdering, cache *pb.OrderCache) string {

	for _, props := range o.GetProperties() {
		if props.GetFolderNumber() == built.GetFromFolder() {
			for _, ordering := range cache.GetCache()[built.GetInstanceId()].GetOrderings() {
				if ordering.GetOrdering() == props.GetOrder() {
					return fmt.Sprintf("%v-%v", props.GetIndex(), ordering.GetOrderString())
				}
			}
		}
	}

	key, err := utils.GetContextKey(ctx)
	s.RaiseIssue("Ordering Cache Miss", fmt.Sprintf("%v has not ordering from %v and %v : %v/%v", built.GetInstanceId(), built, cache.GetCache()[built.GetInstanceId()], key, err))
	return ""
}
