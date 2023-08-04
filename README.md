# Google Kubernetes Engine (GKE) Cluster

This example deploys multiple Google Cloud Platform (GCP) [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) cluster in [Autopilot Mode](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview). 

It then constructs a [Google External L7 Load Balancer](https://cloud.google.com/load-balancing/docs/https) with [Serverless NEGs (Network Endpoint Groups)](https://cloud.google.com/load-balancing/docs/negs/serverless-neg-concepts). 

Additionally a Domain Name is linked to a reserved static IP address and joined to the External Balancer providing a signed SSL Certificate. The load balancer and SSL is then bound to the SNEGs and in turn registered with all GKE Clusters. 

This provides a multi-region load balanced set of GKE Clusters. In this demo all clusters run identical worloads configured using [Helm Charts](https://helm.sh/) which are also deployed with the [Pulumi Kubernetes/Helm provider](https://www.pulumi.com/registry/packages/kubernetes/api-docs/helm/). 

The GKE Clusters are additionally configured with [Managed ASM (Anthos Service Mesh)](https://cloud.google.com/service-mesh/docs/managed/provision-managed-anthos-service-mesh) to give visibility of the workloads across a multi-cluster mesh. To enable this the clusters are also enrolled into [GKE Fleet Management](https://cloud.google.com/kubernetes-engine/docs/fleets-overview). 


## Deploying the App

To deploy your infrastructure and demo applications, follow the below steps.

### Prerequisites

1. [Install Pulumi](https://www.pulumi.com/docs/get-started/install/)
2. [Install Go 1.20 or later](https://golang.org/doc/install)
1. [Install Google Cloud SDK (`gcloud`)](https://cloud.google.com/sdk/docs/downloads-interactive)
1. Configure GCP Auth

    * Login using `gcloud`

        ```bash
        $ gcloud auth login
        $ gcloud config set project <YOUR_GCP_PROJECT_HERE>
        $ gcloud auth application-default login
        ```
    > Note: This auth mechanism is meant for inner loop developer
    > workflows. If you want to run this example in an unattended service
    > account setting, such as in CI/CD, please [follow instructions to
    > configure your service account](https://www.pulumi.com/docs/intro/cloud-providers/gcp/setup/). The
    > service account must have the role `Kubernetes Engine Admin` / `container.admin`.
1. Locate a Domain Name to which you can use for this demo, ensure you have the ability to change the DNS records for this domain.

### Steps

After cloning this repo, from this working directory, navigate to the ```infra/go``` folder and run these commands:

1. Create a new Pulumi stack, which is an isolated deployment target for this example:

    This will initialize the Pulumi program in Golang.

    ```bash
    $ pulumi stack init
    ```

1. Set the required GCP configuration variables:

    This sets configuration options and default values for our cluster.

    ```bash
    $ pulumi config set prefix <YOUR_CHOSEN_RESOURCE_PREFIX>
    $ pulumi config set gcpProjectId <YOUR_GCP_PROJECT_ID_HERE>
    $ pulumi config set domainName <YOUR_DOMAIN_HERE>     // A domain you own and can control DNS records.
    ```
1. Setup the regions and clusters:
   There is the posibility to configure additional GKE Clusters in additional regions as part of this deployment. 

   By Default; This demo configures a new VPC and in this VPC it will, by default, create two regional subnets:

   - "us-central1"
   - "europe-west6"

   Each subnet region will also host a new GKE Cluster which is linked to our Global Load Balancer and has our Helm Charts deployed to it. 

   To make modifications easy we have pre-provisioned additional subnets and clusters but marked them in the configuration as ```enabled: false```. 

   You can explore and make modifications to this code block to add, enable, disable or remove additional GKE Clusters, Regions and Associated Subnets.

   ```
   var CloudRegions = []cloudRegion{
	cloudRegion{
		Id:                 "001",
		Enabled:            true,
		Region:             "us-central1",
		SubnetIp:           "10.128.50.0/24",
		KubernetesProvider: &k8s.Provider{},
	},
	cloudRegion{
		Id:                 "002",
		Enabled:            true,
		Region:             "europe-west6",
		SubnetIp:           "10.128.100.0/24",
		KubernetesProvider: &k8s.Provider{},
	},

    ....
    ....

}
   ```

1. Stand up the Infrasture & Deploy Applications:

    Now that we have configured which regions and how many clusters to provision it is time to stand up your infrastructure with Pulumi. 
    
    To preview and deploy changes, run `pulumi up` and select "yes."

    The `up` sub-command shows a preview of the resources that will be created
    and prompts on whether to proceed with the deployment. Note that the stack
    itself is counted as a resource, though it does not correspond
    to a physical cloud resource.

    You can also run `pulumi up --diff` to see and inspect the diffs of the
    overall changes expected to take place.

    Running `pulumi up` will deploy the GKE clusters and associated netowrking. Note, provisioning a
    new GKE cluster takes between 5-7 minutes per cluster. Additional time will be required for the networking and application Helm deployments.

   

1. After 3-5 minutes, your cluster will be ready, and the kubeconfig JSON you'll use to connect to the cluster will
   be available as an output.

1. Access the Kubernetes Cluster using `kubectl`

    To access your new Kubernetes cluster using `kubectl`, we need to setup the
    `kubeconfig` file and download `kubectl`. We can leverage the Pulumi
    stack output in the CLI, as Pulumi facilitates exporting these objects for us.

    ```bash
    $ pulumi stack output kubeconfig --show-secrets > kubeconfig
    $ export KUBECONFIG=$PWD/kubeconfig

    $ kubectl version
    $ kubectl cluster-info
    $ kubectl get nodes
    ```

1. Once you've finished experimenting, tear down your stack's resources by destroying and removing it:

    ```bash
    $ pulumi destroy --yes
    $ pulumi stack rm --yes
    ```