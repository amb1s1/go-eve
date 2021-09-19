# go-eve

## Requirements

1. Set up the gcloud cli to run on your pc (instructions: https://cloud.google.com/sdk/gcloud)
2. Create a service account for your go application to run under (instructions: https://cloud.google.com/docs/authentication/production#creating_a_service_account)
3. Grant permissions to the service account (use the same instructions link above)
4. Create a local key file containing your new service account credentials (use the same instructions link above)
5. Set the GOOGLE_APPLICATION_CREDENTIALS environment variable to the path of your local key file
