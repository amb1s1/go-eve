// Copyright 2017 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/melbahja/goph"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var (
	projectID              = flag.String("project", "", "name of your project")
	instanceName           = flag.String("instance_name", "", "name of your compute instance")
	zone                   = flag.String("zone", "us-central1-a", "default to us-central1-a zone")
	sshPublicKeyFileName   = flag.String("ssh_public_key_file_name", "", "path to the file containing your ssh public key.")
	sshPrivateKeyFileName  = flag.String("ssh_private_key_file_name", "", "path to the file containing your ssh private key.")
	sshKeyUsername         = flag.String("ssh_key_username", "", "use the username from your ssh public key. If appear on your ssh public key file. If not do not use this flag. E.g user@domain.")
	createCustomEveNGImage = flag.Bool("create_custom_eve_ng_image", false, "Create a custom eve-ng image if not already created")
	customEveNGImageName   = flag.String("custom_eve_ng_image_name", "eve-ng", "Create a custom eve-ng image if not already created. Default is eve-ng")

	bashFiles = []string{"install.sh", "eve-initial-setup.sh"}
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
	sshKey := readSSHKey()

	instance := &compute.Instance{
		Name:           *instanceName,
		Description:    "compute sample instance",
		MinCpuPlatform: "Intel Cascade Lake",
		MachineType:    prefix + "/zones/" + *zone + "/machineTypes/c2-standard-4",
		CanIpForward:   true,
		Tags: &compute.Tags{
			Items: []string{
				"http-server",
				"https-server",
			},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "my-root-" + *instanceName,
					SourceImage: "projects/amb1s1/global/images/" + *customEveNGImageName,
					DiskType:    "projects/amb1s1/zones/us-central1-a/diskTypes/pd-ssd",
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
					Value: proto.String(*sshKeyUsername + ":" + string(sshKey)),
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

func natIPLookup(service *compute.Service) (net.Addr, error) {
	time.Sleep(60 * time.Second)
	log.Println("looking for the instance NatIP")
	found, _ := service.Instances.Get(*projectID, *zone, *instanceName).Do()
	for _, i := range found.NetworkInterfaces {
		nat, err := net.ResolveIPAddr("ip", i.AccessConfigs[len(i.AccessConfigs)-1].NatIP)
		if err != nil {
			return nil, err
		}
		return nat, nil
	}
	return nil, nil
}

func createInstance(ctx context.Context, service *compute.Service) {
	instanceRequest := contructInstanceRequest()
	op, err := service.Instances.Insert(*projectID, *zone, instanceRequest).Do()
	if err != nil {
		log.Printf("Got compute.Operation, err: %#v, %v", op, err)
	}
	etag := op.Header.Get("Etag")
	inst, err := service.Instances.Get(*projectID, *zone, *instanceName).IfNoneMatch(etag).Do()
	if err != nil {
		log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
	}
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}

}

func createEveNGImage(s *compute.Service) {
	image := &compute.Image{
		Name: *customEveNGImageName,
		Licenses: []string{
			"https://www.google.com/compute/v1/projects/vm-options/global/licenses/enable-vmx",
		},
		SourceDisk: "https://www.googleapis.com/compute/beta/projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20210429",
		DiskSizeGb: 10,
	}
	_, err := s.Images.Insert(*projectID, image).Do()
	if err != nil {
		log.Fatalf("failed to create a new image error: %v", err)
	}
}

func readSSHKey() []byte {
	body, err := ioutil.ReadFile(*sshPublicKeyFileName)
	if err != nil {
		log.Fatalf("could not read public ssh file, error: %v", err)
	}
	return body
}

func sshToServer(natIP net.Addr) *goph.Client {
	log.Printf("ssh to: %v", natIP)
	// Start new ssh connection with private key.
	priKey, err := goph.Key(*sshPrivateKeyFileName, "")
	if err != nil {
		log.Fatalf("could not get the ssh private key, error: %v", err)
	}

	client, err := goph.NewUnknown(*sshKeyUsername, natIP.String(), priKey)
	if err != nil {
		log.Fatalf("could not create a new ssh client, error: %v", err)
	}
	return client

}

func fetchScript(client *goph.Client) error {
	dir, _ := os.Getwd()
	for _, f := range bashFiles {
		err := client.Upload(dir+"/"+f, "/home/"+*sshKeyUsername+"/"+f)
		if err != nil {
			return err
		}
	}
	return nil
}

func runRemoteScript(client *goph.Client, f string) error {
	// Execute your command.
	log.Println("runnning script...")
	out, err := client.Run("chmod +x /home/" + *sshKeyUsername + "/" + f)
	if err != nil {
		return err
	}
	log.Println(string(out))
	out, err = client.Run("sudo /home/" + *sshKeyUsername + "/" + f)
	if err != nil {
		return err
	}
	log.Println(string(out))

	return nil
}

func reboot(client *goph.Client) error {
	log.Println("rebooting....")
	out, err := client.Run("sudo reboot -f")
	log.Printf("Reboot Out: %v", string(out))
	if err != nil {
		return err
	}
	return nil
}

func initialSetup(service *compute.Service, natIP net.Addr) error {
	for _, f := range bashFiles {
		client := sshToServer(natIP)
		defer client.Close()
		err := fetchScript(client)
		if err != nil {
			log.Println(err)
		}
		err = runRemoteScript(client, f)
		if err != nil {
			return err
		}
		reboot(client)
		time.Sleep(60 * time.Second)
	}
	return nil
}

func main() {
	flag.Parse()
	ctx := context.Background()
	service, err := initService(ctx)
	if err != nil {
		log.Fatalf("Not able to create a compute service, error: %v", err)
	}
	if *createCustomEveNGImage {
		createEveNGImage((service))
	}

	exists := isInstanceExists(service)
	if exists {
		log.Fatalf("Instance %v already exists", *instanceName)
	}
	createInstance(ctx, service)

	natIP, err := natIPLookup(service)
	if err != nil {
		log.Fatalf("could not get instance external ip, error: %v", err)
	}
	err = initialSetup(service, natIP)
	if err != nil {
		log.Println(err)
	}
	if err != nil {
		log.Println(err)
	}
}
