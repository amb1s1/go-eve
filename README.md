
# go-eve 


## Summary
go-eve is trying to solve every Network Engineer's problem when they start working on a network lab; it is time-consuming. Unfortunately, it takes a long time to learn, set up, build and maintain their lab environment.  

Go-eve is a tool for the EVE-NG network simulator platform.

The goal is that every time we run our tool, the lab's state will be as intended. If a lab is not working as expected, we can destroy the lab, rebuild it, and the lab will be as expected.

This tool will :
1. Build: Build your compute instance ( Initial will support Google Cloud)
2. Setup: Setup the cloud instance with all the software and setting needed for your initial EVE-NG lab. 
3. Destroy: Will destroy your entire lab.

## Requirements

1. Set up the gcloud cli to run on your pc (instructions: https://cloud.google.com/sdk/gcloud)
  1. gcl cli install guide: https://cloud.google.com/sdk/docs/install     
2. Create a service account for your go application to run under (instructions: https://cloud.google.com/docs/authentication/production#creating_a_service_account)
3. Grant permissions to the service account (use the same instructions link above)
4. Create a local key file containing your new service account credentials (use the same instructions link above)
5. Set the GOOGLE_APPLICATION_CREDENTIALS environment variable to the path of your local key file

![alt text](https://golang.org/lib/godoc/images/go-logo-blue.svg)
