package main

import (
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver/utils"

	pbrc "github.com/brotherlogic/recordcollection/proto"
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

	case "fullping":
		ctx2, cancel2 := utils.ManualContext("recordcollectioncli-"+os.Args[1], time.Hour)
		defer cancel2()

		conn2, err := utils.LFDialServer(ctx2, "recordcollection")
		if err != nil {
			log.Fatalf("Cannot reach rc: %v", err)
		}
		defer conn2.Close()

		registry := pbrc.NewRecordCollectionServiceClient(conn2)
		ids, err := registry.QueryRecords(ctx2, &pbrc.QueryRecordsRequest{Query: &pbrc.QueryRecordsRequest_All{true}})
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
