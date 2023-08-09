# Google Kubernetes Engine (GKE) Cluster

This example deploys a set of multi-region Google Cloud Platform (GCP) [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) clusters in [Autopilot Mode](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview) using [Pulumi](https://pulumi.com).

It then constructs a [Google External L7 Load Balancer](https://cloud.google.com/load-balancing/docs/https) with [Serverless NEGs (Network Endpoint Groups)](https://cloud.google.com/load-balancing/docs/negs/serverless-neg-concepts).

Additionally a Domain Name is linked to a reserved static IP address and joined to the External Balancer providing a signed SSL Certificate. The load balancer and SSL is then bound to the SNEGs and in turn registered with all GKE Clusters.

This provides a multi-region load balanced set of GKE Clusters. In this demo all clusters run identical workloads configured using [Helm Charts](https://helm.sh/) which are also deployed with the [Pulumi Kubernetes/Helm provider](https://www.pulumi.com/registry/packages/kubernetes/api-docs/helm/).

The GKE Clusters are additionally configured with [Managed Istio (Anthos Service Mesh)](https://istio.io/latest/about/service-mesh/) to give visibility of the workloads across a multi-cluster mesh. To enable this the clusters are also enrolled into [GKE Fleet Management](https://cloud.google.com/kubernetes-engine/docs/fleets-overview).

## The Design

### High Level Design

![Google Cloud High Level Infrastructure Diagram](https://github.com/timbohiatt/gke-at-scale-pulumi/blob/main/docs/001-google-cloud-infra.png?raw=true)

### Load Balancer Breakdown

When deploying a Google Cloud load balancer there is lots of configuration to consider. In our deployment we will be deploying a Layer 7 External Load Balancer. This loa balancer breaks down into several components illustrated in the diagram below.

![Google Cloud Design - Load Balancer Breakdown](https://github.com/timbohiatt/gke-at-scale-pulumi/blob/main/docs/002-load-balancer-breakdown.png?raw=true)

1. External static IP Address; Reserved and assigned as the entry to point to the global load balancer. This static IP address is assigned to your DNS Provider for validation for the SSL Certificate. 
2. Forwarding Rule; Responsible for forwarding TCP traffic that enters the Google Cloud Network via the static IP address to a given HTTP or HTTPS Target Proxy. In our deployment we deploy two forwarding rules.
    - 1x HTTPS TCP traffic on 443 
    - 1x HTTP TCP traffic on port 80
3. Target Proxies; Responsible for linking the SSL certificates with a URL map that determines how traffic will be routed through the Load Balancer to Google Cloud Compute backends. 
4. URL Map; Reads the L7 TCP headers and compares them against the URL Maps associated with the Target Proxies. The URL map contains the configuration that determines which URL domains and paths will route to which Google Compute Backend services (VM's, Serverless, GKE). In our deployment we configure a single URL map for our chosen domain and all traffic routed into this URL Map from the Target Proxies will be routed to our GKE Backend Services.
5. Backend Services are the bridge between the Load Balancer and the target compute; In our deployment our traffic will be routed from the URL map to a single Backend Service which will contain as series GKE Pod IP addresses (Network Endpoints) which will exist in a NEG (Network Endpoint Group).

## Deploying the App

To deploy your infrastructure and demo applications, follow the below steps.

### Prerequisites

1. [Install Pulumi](https://www.pulumi.com/docs/get-started/install/)
1. [Install Go 1.20 or later](https://golang.org/doc/install)
1. [Install Google Cloud SDK (`gcloud`)](https://cloud.google.com/sdk/docs/downloads-interactive)
1. Configure GCP Auth with `gcloud`:

    ```bash
    gcloud auth login
    gcloud config set project <YOUR_GCP_PROJECT_HERE>
    gcloud auth application-default login
    ```

    **Note:** This auth mechanism is meant for inner loop developer workflows. If you want to run this example in an unattended service account setting, such as in CI/CD, see [Using a Service Account](https://www.pulumi.com/docs/intro/cloud-providers/gcp/setup/) in the Pulumi docs. The service account must have the role `Kubernetes Engine Admin` / `container.admin`.
1. A Domain Name to which you can use for this demo, ensure you have the ability to change the DNS records for this domain.

### Steps

After cloning this repo, from this working directory, navigate to the ```infra/go``` folder and run these commands:

1. Create a new Pulumi stack, which is an isolated deployment target for this example:

    This will initialize the Pulumi program in Golang.

    ```bash
    pulumi stack init
    ```

1. Set the required GCP configuration variables. This sets configuration options and default values for our cluster:

    ```bash
    pulumi config set prefix <YOUR_CHOSEN_RESOURCE_PREFIX>
    pulumi config set gcpProjectId <YOUR_GCP_PROJECT_ID_HERE>
    pulumi config set domainName <YOUR_DOMAIN_HERE>     # A domain you own and can control DNS records.
    ```

1. Setup the regions and clusters:
    There is the possibility to configure additional GKE Clusters in additional regions as part of this deployment.

    By Default; This demo configures a new VPC and in this VPC it will, by default, create two regional subnets:

    1. `us-central1`
    1. `europe-west6`

    Each subnet region will also host a new GKE Cluster which is linked to our Global Load Balancer and has our Helm Charts deployed to it.

    To make modifications easy we have pre-provisioned additional subnets and clusters but marked them in the configuration as `enabled: false`.

    You can explore and make modifications to this code block to add, enable, disable or remove additional GKE Clusters, Regions and Associated Subnets.

    ```go
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
    // etc
    }
    ```

1. Stand up the Infrastructure & Deploy Applications:

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

    ```bash
    pulumi update
    ```

1. You can access the Kubeconfig for the generated clusters via the following command:

    ```bash
    pulumi output KubeConfig
    ```

1. Once you've finished experimenting, tear down your stack's resources by destroying and removing it:

    ```bash
    pulumi destroy --yes
    pulumi stack rm --yes
    ```
