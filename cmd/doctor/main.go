package main

import (
	"log"
	"net/http"

	"google.golang.org/grpc"

	"github.com/iryonetwork/network-poc/client"
	"github.com/iryonetwork/network-poc/config"
	"github.com/iryonetwork/network-poc/specs"
	"github.com/iryonetwork/network-poc/storage/ehr"
	"github.com/iryonetwork/network-poc/storage/eth"
)

func main() {
	config, err := config.New()
	if err != nil {
		log.Fatalf("failed to get config: %v", err)
	}
	config.ClientType = "Doctor"

	eth := eth.New(config)
	ehr := ehr.New()

	conn, err := grpc.Dial(config.IryoAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	client, err := client.New(config, specs.NewCloudClient(conn), eth, ehr)
	if err != nil {
		log.Fatalf("failed to initialize client: %v", err)
	}
	err = client.Subscribe()
	if err != nil {
		log.Fatalf("failed to subscribe to events: %v", err)
	}

	h := &handlers{
		config: config,
		client: client,
		ehr:    ehr,
	}

	http.HandleFunc("/ehr/", h.ehrHandler)
	http.HandleFunc("/save", h.saveEHRHandler)
	http.HandleFunc("/", h.indexHandler)

	log.Printf("starting HTTP server on http://%s", config.ClientAddr)

	if err := http.ListenAndServe(config.ClientAddr, nil); err != nil {
		log.Fatalf("error serving HTTP: %v", err)
	}
}
