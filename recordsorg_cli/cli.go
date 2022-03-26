package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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

	switch os.Args[1] {
	case "get":
		getFlags := flag.NewFlagSet("get", flag.ExitOnError)
		var name = getFlags.String("name", "", "The name of the budgorget")
		if err := getFlags.Parse(os.Args[2:]); err == nil {

			client := pb.NewRecordsOrgServiceClient(conn)
			resp, err := client.GetOrg(ctx, &pb.GetOrgRequest{OrgName: *name})
			if err != nil {
				log.Fatalf("Cannot get: %v", err)
			}

			fmt.Printf("Got %v\n", resp.GetOrg().GetName())
			for _, order := range resp.GetOrg().GetOrderings() {
				fmt.Printf("%v. %v\n", order.GetIndex(), order.GetInstanceId())
			}
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
