// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"

	"github.com/golang/protobuf/proto"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var (
	projectID      = flag.String("project", "", "name of your project")
	instanceName   = flag.String("instance_name", "", "name of your compute instance")
	zone           = flag.String("zone", "us-central1-a", "default to us-central1-a zone")
	sshKeyFileName = flag.String("ssh_key_file_name", "", "path to the file containing your ssh public key.")
	sshKeyUsername = flag.String("ssh_key_username", "", "use the username from your ssh public key. If appear on your ssh public key file. If not do not use this flag. E.g user@domain.")
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
	sshKey, err := getLocalSSHKey(*sshKeyFileName)
	if err != nil {
		log.Printf("could not get your ssh public key from file name: %v", *sshKeyFileName)
	}

	instance := &compute.Instance{
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
		Metadata: &compute.Metadata{
			Kind: "compute#metadata",
			Items: []*compute.MetadataItems{
				{
					Key:   "ssh-keys",
					Value: proto.String(sshKey),
				},
			},
		},
	}
	return instance

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

func getLocalSSHKey(f string) (string, error) {
	body, err := ioutil.ReadFile(*sshKeyFileName)
	if err != nil {
		return "", err
	}
	return *sshKeyUsername + ":" + string(body), nil
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
