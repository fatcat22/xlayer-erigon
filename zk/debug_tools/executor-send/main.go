package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/ledgerwatch/erigon/zk/legacy_executor_verifier/proto/github.com/0xPolygonHermez/zkevm-node/state/runtime/executor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var file string
var fileType string
var endpoint string

func main() {
	flag.StringVar(&file, "file", "", "file to send")
	flag.StringVar(&fileType, "file-type", "proto", "file type: json or proto (default)")
	flag.StringVar(&endpoint, "endpoint", "", "endpoint to send to")
	flag.Parse()

	var err error

	contents, err := os.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		return
	}

	var payload executor.ProcessStatelessBatchRequestV2

	if fileType == "proto" {
		err = proto.UnmarshalText(string(contents), &payload)
	} else if fileType == "json" {
		err = json.Unmarshal(contents, &payload)
	} else {
		fmt.Println("File type must be either json or proto.")
		return
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	// Send payload to endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		fmt.Println(err)
		return
	}

	client := executor.NewExecutorServiceClient(conn)

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer sendCancel()

	res, err := client.ProcessStatelessBatchV2(sendCtx, &payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	fullJson, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}

	readWritesJson, err := json.MarshalIndent(res.ReadWriteAddresses, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("---------------------------------------")
	fmt.Printf("full response: %s\n", string(fullJson))
	fmt.Println("---------------------------------------")
	fmt.Println("writes:")
	fmt.Println(string(readWritesJson))
	fmt.Println("---------------------------------------")
	fmt.Printf("new root: 0x%x\n", res.NewStateRoot)
	fmt.Printf("old root: 0x%x\n", res.OldStateRoot)
	fmt.Printf("error: %s\n", res.Error)
	fmt.Printf("rom error: %s\n", res.ErrorRom)
}
