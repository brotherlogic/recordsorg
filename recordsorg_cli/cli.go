package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/brotherlogic/goserver/utils"

	pbrc "github.com/brotherlogic/recordcollection/proto"
	pb "github.com/brotherlogic/recordsorg/proto"
)

func main() {
	ctx, cancel := utils.ManualContext("recordsorg_cli", time.Minute*30)
	defer cancel()

	conn, err := utils.LFDialServer(ctx, "recordsorg")
	if err != nil {
		log.Fatalf("Unable to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRecordsOrgServiceClient(conn)

	switch os.Args[1] {
	case "reorg":
		getFlags := flag.NewFlagSet("get", flag.ExitOnError)
		var name = getFlags.String("name", "", "The name of the budgorget")
		if err := getFlags.Parse(os.Args[2:]); err == nil {
			_, err = client.Reorg(ctx, &pb.ReorgRequest{OrgName: *name})
			if err != nil {
				log.Fatalf("Bad org: %v", err)
			}
		}
	case "get":
		getFlags := flag.NewFlagSet("get", flag.ExitOnError)
		var name = getFlags.String("name", "", "The name of the budgorget")
		if err := getFlags.Parse(os.Args[2:]); err == nil {
			conn2, err := utils.LFDialServer(ctx, "recordcollection")
			if err != nil {
				log.Fatalf("Cannot get: %v", err)
			}
			defer conn2.Close()
			registry := pbrc.NewRecordCollectionServiceClient(conn2)

			resp, err := client.GetOrg(ctx, &pb.GetOrgRequest{OrgName: *name})
			if err != nil {
				log.Fatalf("Cannot get: %v", err)
			}

			fmt.Printf("Got %v\n", resp.GetOrg().GetName())
			sort.SliceStable(resp.GetOrg().Orderings, func(i, j int) bool {
				return resp.GetOrg().GetOrderings()[i].GetIndex() < resp.GetOrg().GetOrderings()[j].GetIndex()
			})
			total := float32(0)
			for _, order := range resp.GetOrg().GetOrderings() {
				record, err := registry.GetRecord(ctx, &pbrc.GetRecordRequest{InstanceId: order.GetInstanceId()})
				if err != nil {
					log.Fatalf("Bad get: %v", err)
				}
				fmt.Printf("%v. %v. [%v] %v - %v [%v] = %v\n",
					order.GetSlotNumber(),
					order.GetIndex(),
					order.GetInstanceId(),
					record.GetRecord().GetRelease().GetArtists()[0].GetName(),
					record.GetRecord().GetRelease().GetTitle(),
					order.GetTakenWidth(),
					order.GetOrdered())
				total += order.GetTakenWidth()
			}
			fmt.Printf("Total Width = %v", total)
		}
	case "ping":
		getFlags := flag.NewFlagSet("get", flag.ExitOnError)
		var id = getFlags.Int("id", -1, "The name of the budgorget")
		if err := getFlags.Parse(os.Args[2:]); err == nil {
			sclient := pbrc.NewClientUpdateServiceClient(conn)
			log.Printf("PING %v", *id)
			ctx, cancel = utils.ManualContext("recordsorg-cli-fullping", time.Minute*30)
			_, err = sclient.ClientUpdate(ctx, &pbrc.ClientUpdateRequest{InstanceId: int32(*id)})
			cancel()
			if err != nil {
				log.Fatalf("Error on GET: %v", err)
			}
		}
	case "fullping":
		ctx2, cancel2 := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour)
		defer cancel2()

		conn2, err := utils.LFDialServer(ctx2, "recordcollection")
		if err != nil {
			log.Fatalf("Cannot reach rc: %v", err)
		}
		defer conn2.Close()

		registry := pbrc.NewRecordCollectionServiceClient(conn2)
		ids, err := registry.QueryRecords(ctx2, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{All: true}})
		if err != nil {
			log.Fatalf("Bad query: %v", err)
		}

		sclient := pbrc.NewClientUpdateServiceClient(conn)
		for i, id := range ids.GetInstanceIds() {
			log.Printf("PING %v -> %v", i, id)
			ctx, cancel = utils.ManualContext("recordsorg-cli-fullping", time.Minute*30)
			_, err = sclient.ClientUpdate(ctx, &pbrc.ClientUpdateRequest{InstanceId: int32(id)})
			cancel()
			if err != nil {
				log.Fatalf("Error on GET: %v", err)
			}
		}
	}
}
