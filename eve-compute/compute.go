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
	IsImageCreated(string, string) bool
	CreateImage(string, *compute.Image) error
	DeleteImage(string, string) error
	CreateInstance(string, string, *compute.Instance) error
	InsertFireWallRule(string, *compute.Firewall) error
	DeleteFirewallRules(string) error
	GetExternalIP(string, string, string) (net.Addr, error)
	InstanceStatus(string, string, string) string
	DeleteInstance(string, string, string) error
	StopInstance(string, string, string) error
	StartInstance(string, string, string) error
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

func (c computeService) IsImageCreated(projectID, imageName string) bool {
	op, err := c.service.Images.Get(projectID, imageName).Do()
	if err != nil {
		return false
	}
	if op.Status == "READY" {
		return true
	}
	return false
}

func (c computeService) CreateImage(projectID string, image *compute.Image) error {
	_, err := c.service.Images.Insert(projectID, image).Do()
	if err != nil {
		return err
	}

	for {
		time.Sleep(10 * time.Second)
		op, _ := c.service.Images.Get(projectID, image.Name).Do()
		log.Printf("Operation: %v", op.Status)
		log.Printf("creating new image %v status is: %v", image.Name, op.Status)
		if op.Status != "PENDING" {
			break
		}
	}

	return nil
}

func (c computeService) DeleteImage(projectID, imageName string) error {
	_, err := c.service.Images.Delete(projectID, imageName).Do()
	if err != nil {
		return err
	}
	log.Printf("Deleted image: %v", imageName)
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

func (c computeService) DeleteFirewallRules(projectID string) error {
	for _, f := range []string{"ingress-eve", "egress-eve"} {
		log.Printf("Deleting firewall rule: %v", f)
		_, err := c.service.Firewalls.Delete(projectID, f).Do()
		if err != nil {
			return err
		}
		log.Printf("Deleted firewall rule: %v", f)

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

func (c computeService) InstanceStatus(projectID, zone, instanceName string) string {
	found, _ := c.service.Instances.Get(projectID, zone, instanceName).Do()
	if found != nil {
		return found.Status
	}
	return ""
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
		if status := c.InstanceStatus(projectID, zone, instanceName); status == "" {
			break
		}
	}
	log.Println(op.Status)
	return nil
}

func (c computeService) StopInstance(projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Stop(projectID, zone, instanceName).Do()
	if err != nil {
		return err
	}
	for {
		log.Printf("Stopping instance %v", instanceName)
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()                                                   // Start the spinner
		time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
		s.Stop()
		if status := c.InstanceStatus(projectID, zone, instanceName); status == "TERMINATED" {
			log.Printf("Instance %v stopped", instanceName)
			break
		}
	}
	return nil
}

func (c computeService) StartInstance(projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Start(projectID, zone, instanceName).Do()
	if err != nil {
		return err
	}
	for {
		log.Printf("Starting instance %v", instanceName)
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()                                                   // Start the spinner
		time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
		s.Stop()
		if status := c.InstanceStatus(projectID, zone, instanceName); status == "RUNNING" {
			log.Printf("Instance %v is running", instanceName)
			break
		}
	}
	return nil
}
