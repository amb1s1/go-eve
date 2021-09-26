
# go-eve BUILD, PLAY, TEARDOWN, REPEAT


## Summary
go-eve is trying to solve every Network Engineer's problem when they start working on a network lab; it is time-consuming. Unfortunately, it takes a long time to learn, set up, build and maintain their lab environment.  

Go-eve is a tool for the EVE-NG network simulator platform.

The goal is that every time we run our tool, the lab's state will be as intended. If a lab is not working as expected, we can destroy the lab, rebuild it, and the lab will be as expected.

This tool will :
1. Build: Build your compute instance ( Initial will support Google Cloud)
2. Setup: Setup the cloud instance with all the software and setting needed for your initial EVE-NG lab. 
3. Destroy: Will destroy your entire lab.

## Requirements

1. Golang install
2. Set up the gcloud CLI to run on your computer [gcl CLI install guide](https://cloud.google.com/sdk/docs/install)     
3. Create a Project
5. Enable the Compute Engine and Cloud Build APIs.
    [Enable the APIs](https://console.cloud.google.com/flows/enableapi?apiid=compute,cloudbuild.googleapis.com&_ga=2.208966098.1574923679.1632600072-1712777355.1631763170)
5. Create a local key file containing your new service account credentials (use the same instructions link above)
6. Set the GOOGLE_APPLICATION_CREDENTIALS environment variable to the path of your local key file.
7. Initiate gcloud SDK `gcloud init` [guide](https://cloud.google.com/sdk/gcloud/reference/init)



## How to use it
### Configuration
1. Make sure that you ran `gcloud init` to set up your project.
2. Open the `config.yaml` file and make all the necessary changes.

### Build it
`go build main.go`

### run
On your first run, you will need to create a custom eve-ng image.
`./main --create_custom_eve_ng_image=true --instance_name=eve-go1`
This will:
1. Build a custom image.
2. Install eve-ng.
3. Setup eve-ng.
4. Create an Ingress && Egress firewall to allow telnet to the eve-ng node from outside gcloud.

In the end, you should be able to HTTP into the eve-ng server and create labs.


## Roadmap
1. Upload and setup by vendor and os(You still need to provide the os).
2. Backup your lab config.
