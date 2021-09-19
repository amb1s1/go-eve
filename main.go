// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"log"

	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var (
	projectID    = flag.String("project", "", "name of your project")
	instanceName = flag.String("instance_name", "", "name of your compute instance")
	zone         = flag.String("zone", "us-central1-a", "default to us-central1-a zone")
)

func initService(ctx context.Context) (*compute.Service, error) {
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}
	return service, err
}

func contructInstanceRequest() *compute.Instance {
	prefix := "https://www.googleapis.com/compute/v1/projects/" + *projectID
	imageURL := "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-2104-hirsute-v20210909"

	return &compute.Instance{
		Name:        *instanceName,
		Description: "compute sample instance",
		MachineType: prefix + "/zones/" + *zone + "/machineTypes/n1-standard-1",
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "my-root-" + *instanceName,
					SourceImage: imageURL,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: prefix + "/global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.DevstorageFullControlScope,
					compute.ComputeScope,
				},
			},
		},
	}

}

func isInstanceExists(service *compute.Service) bool {
	found, _ := service.Instances.Get(*projectID, *zone, *instanceName).Do()
	if found != nil {
		return true
	}
	return false

}
func createInstance(ctx context.Context, service *compute.Service) {
	instanceRequest := contructInstanceRequest()
	op, err := service.Instances.Insert(*projectID, *zone, instanceRequest).Do()
	log.Printf("Got compute.Operation, err: %#v, %v", op, err)
	etag := op.Header.Get("Etag")
	log.Printf("Etag=%v", etag)

	inst, err := service.Instances.Get(*projectID, *zone, *instanceName).IfNoneMatch(etag).Do()
	log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}
}

func main() {
	flag.Parse()
	ctx := context.Background()
	service, err := initService(ctx)
	if err != nil {
		log.Fatalf("Not able to create a compute service, error: %v", err)
	}

	exists := isInstanceExists(service)
	if exists {
		log.Fatalf("Instance %v already exists", *instanceName)
	}
	createInstance(ctx, service)

}
