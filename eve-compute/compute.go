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
	isFirewallRuleExist(string, string) bool
	IsImageCreated(string, string) bool
	CreateImage(string, *compute.Image) error
	DeleteImage(string, string) error
	CreateInstance(string, string, *compute.Instance) error
	InsertFirewallRule(string, *compute.Firewall) error
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
	log.Printf("creating new image %v.", image.Name)
	_, err := c.service.Images.Insert(projectID, image).Do()
	if err != nil {
		return err
	}

	for {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()
		time.Sleep(10 * time.Second)
		s.Stop()
		op, _ := c.service.Images.Get(projectID, image.Name).Do()
		log.Printf("creating new image %v status is: %v.", image.Name, op.Status)
		if op.Status != "PENDING" {
			break
		}
	}

	return nil
}

func (c computeService) DeleteImage(projectID, imageName string) error {
	if created := c.IsImageCreated(projectID, imageName); created {
		_, err := c.service.Images.Delete(projectID, imageName).Do()
		if err != nil {
			return err
		}
		log.Printf("deleted image: %v.", imageName)
		return nil

	}
	log.Printf("image: %v was not deleted. Image not found.", imageName)
	return nil
}

func (c computeService) CreateInstance(projectID, zone string, instanceRequest *compute.Instance) error {
	log.Printf("creating instance %v.", instanceRequest.Name)
	status := c.InstanceStatus(projectID, zone, instanceRequest.Name)
	if status == "" {
		_, err := c.service.Instances.Insert(projectID, zone, instanceRequest).Do()
		if err != nil {
			return err
		}
		for {
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()
			if status := c.InstanceStatus(projectID, zone, instanceRequest.Name); status == "RUNNING" {
				return nil
			}
		}
	}

	log.Printf("compute instance %v already exist.", instanceRequest.Name)
	return nil
}
func (c computeService) isFirewallRuleExist(projectID, ruleName string) bool {
	_, err := c.service.Firewalls.Get(projectID, ruleName).Do()
	if err != nil {
		return false
	}
	return true
}
func (c computeService) InsertFirewallRule(projectID string, firewallRequest *compute.Firewall) error {
	if created := c.isFirewallRuleExist(projectID, firewallRequest.Name); !created {
		_, err := c.service.Firewalls.Insert(projectID, firewallRequest).Do()
		if err != nil {
			return err
		}
	}
	log.Printf("firewall rule %v already exist.", firewallRequest.Name)
	return nil
}

func (c computeService) DeleteFirewallRules(projectID string) error {
	for _, f := range []string{"ingress-eve", "egress-eve"} {
		if exist := c.isFirewallRuleExist(projectID, f); exist {
			log.Printf("deleting firewall rule: %v.", f)
			_, err := c.service.Firewalls.Delete(projectID, f).Do()
			if err != nil {
				return err
			}
			log.Printf("deleted firewall rule: %v.", f)
			break

		}
		log.Printf("firewall rule: %v was not deleted. Rule not found.", f)

	}
	return nil
}

func (c computeService) GetExternalIP(projectID, zone, instanceName string) (net.Addr, error) {
	log.Println("looking for the instance external ip.")
	found, _ := c.service.Instances.Get(projectID, zone, instanceName).Do()
	for _, i := range found.NetworkInterfaces {
		ip, err := net.ResolveIPAddr("ip", i.AccessConfigs[len(i.AccessConfigs)-1].NatIP)
		if err != nil {
			return nil, err
		}
		log.Printf("got external ip %v for the compute instance %v.", ip, instanceName)
		return ip, nil
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
	if status := c.InstanceStatus(projectID, zone, instanceName); status != "" {
		_, err := c.service.Instances.Delete(projectID, zone, instanceName).Do()
		if err != nil {
			return err
		}
		for {
			log.Printf("deleting instance %v.", instanceName)
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()
			if status := c.InstanceStatus(projectID, zone, instanceName); status == "" {
				return nil
			}
		}
	}
	log.Printf("compute instance %v was not deleted. Instance was not found.", instanceName)
	return nil
}

func (c computeService) StopInstance(projectID, zone, instanceName string) error {
	status := c.InstanceStatus(projectID, zone, instanceName)
	if status == "RUNNING" {
		_, err := c.service.Instances.Stop(projectID, zone, instanceName).Do()
		if err != nil {
			return err
		}
		for {
			log.Printf("stopping instance %v.", instanceName)
			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
			s.Start()                                                   // Start the spinner
			time.Sleep(8 * time.Second)                                 // Run for some time to simulate work
			s.Stop()
			if status := c.InstanceStatus(projectID, zone, instanceName); status == "TERMINATED" {
				log.Printf("instance %v stopped.", instanceName)
				return nil
			}
		}

	}
	if status == "" {
		log.Printf("Compute instance %v was not stop. Instance was not found.", instanceName)
		return nil

	}
	log.Printf("Compute instance %v was not stop. Instance is already shutdown.", instanceName)
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
