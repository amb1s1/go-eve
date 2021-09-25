package evecompute

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

type ServiceFunctions interface {
	CreateImage(string, *compute.Image) error
	CreateInstance(string, string, *compute.Instance) error
	InsertFireWallRule(string, *compute.Firewall) error
	GetExternalIP(string, string, string) (net.Addr, error)
	IsInstanceExists(string, string, string) bool
	DeleteInstance(string, string, string) error
}

type computeService struct {
	service *compute.Service
}

func NewClient() (ServiceFunctions, error) {
	cs := computeService{}
	ctx := context.Background()
	service, err := initService(ctx)
	if err != nil {
		return nil, err
	}
	cs.service = service
	return cs, nil
}

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

func (c computeService) CreateImage(projectID string, image *compute.Image) error {
	_, err := c.service.Images.Insert(projectID, image).Do()
	if err != nil {
		return err
	}
	return nil
}

func (c computeService) CreateInstance(projectID, zone string, instanceRequest *compute.Instance) error {
	_, err := c.service.Instances.Insert(projectID, zone, instanceRequest).Do()
	if err != nil {
		return err
	}
	return nil
}

func (c computeService) InsertFireWallRule(projectID string, firewallRequest *compute.Firewall) error {
	_, err := c.service.Firewalls.Insert(projectID, firewallRequest).Do()
	if err != nil {
		return err
	}
	return nil
}

func (c computeService) GetExternalIP(projectID, zone, instanceName string) (net.Addr, error) {
	time.Sleep(60 * time.Second)
	log.Println("looking for the instance NatIP")
	found, _ := c.service.Instances.Get(projectID, zone, instanceName).Do()
	for _, i := range found.NetworkInterfaces {
		nat, err := net.ResolveIPAddr("ip", i.AccessConfigs[len(i.AccessConfigs)-1].NatIP)
		if err != nil {
			return nil, err
		}
		return nat, nil
	}
	return nil, nil
}

func (c computeService) IsInstanceExists(projectID, zone, instanceName string) bool {
	found, _ := c.service.Instances.Get(projectID, zone, instanceName).Do()
	if found != nil {
		return true
	}
	return false
}

func (c computeService) DeleteInstance(projectID, zone, instanceName string) error {
	op, err := c.service.Instances.Delete(projectID, zone, instanceName).Do()
	if err != nil {
		return err
	}
	for {
		log.Printf("deleting instance %v", instanceName)
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()                                                   // Start the spinner
		time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
		s.Stop()
		if !c.IsInstanceExists(projectID, zone, instanceName) {
			break
		}
	}
	log.Println(op.Status)
	return nil
}
