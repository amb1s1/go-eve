package connect

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/melbahja/goph"
)

type connectFunctons interface {
	Fetch(string) error
	RunScript(string) ([]byte, error)
	Reboot() error
}

// Client represents a ssh gph.Client.
type Client struct {
	ip         net.Addr
	publicKey  string
	privateKey string
	username   string
	Service    *goph.Client
}

// NewClient construct a new ssh client connetion.
func NewClient(publicKey, privateKey, username string, ip net.Addr) (connectFunctons, error) {
	s, err := connect(privateKey, username, ip)
	if err != nil {
		return nil, err
	}

	c := Client{
		publicKey:  publicKey,
		privateKey: privateKey,
		username:   username,
		ip:         ip,
		Service:    s,
	}

	return c, nil
}

func connect(privateKey, username string, ip net.Addr) (*goph.Client, error) {
	// Start new ssh connection with private key.
	priKey, err := goph.Key(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("Could not get privateKey: %v error: %v", privateKey, err)
	}

	c := 0
	for {
		log.Printf("Ssh to: %v", ip)

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Build our new spinner
		s.Start()
		time.Sleep(20 * time.Second)
		s.Stop()

		client, err := goph.NewUnknown(username, ip.String(), priKey)
		if err != nil {
			c += 1
		} else {
			log.Printf("Connected to: %v", ip.String())
			return client, nil
		}

		if c >= 3 {
			return nil, fmt.Errorf("Could not connect to %v, error: %v", ip.String(), err)
		}
	}
}

// Fetch handles uploading files to the remote server.
func (c Client) Fetch(file string) error {
	dir, _ := os.Getwd()

	log.Printf("Fetching file %v to server %v", file, c.ip.String())

	if err := c.Service.Upload(dir+"/"+file, "/home/"+c.username+"/"+file); err != nil {
		return fmt.Errorf("could not fetch file %v, error: %v", file, err)
	}

	log.Printf("Fetched file: %v", file)

	return nil
}

// RunScript runs script on the remote compute instance.
func (c Client) RunScript(file string) ([]byte, error) {
	// Execute your command.
	log.Printf("Making %v executable.", file)

	_, err := c.Service.Run("chmod +x /home/" + c.username + "/" + file)
	if err != nil {
		return nil, err
	}

	log.Printf("Runnning script on file %v", file)

	out, err := c.Service.Run("sudo /home/" + c.username + "/" + file)
	if err != nil {
		return nil, err
	}

	return out, nil
}

//Reboot handles the rebooting of the remote compute instance.
func (c Client) Reboot() error {
	log.Println("Rebooting.")

	out, err := c.Service.Run("sudo reboot -f")
	if err != nil {
		return err
	}

	log.Printf("Reboot status: %v", string(out))
	return nil
}
