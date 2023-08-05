# Google Kubernetes Engine (GKE) Cluster

This example deploys multiple Google Cloud Platform (GCP) [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) cluster in [Autopilot Mode](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview). 

It then constructs a [Google External L7 Load Balancer](https://cloud.google.com/load-balancing/docs/https) with [Serverless NEGs (Network Endpoint Groups)](https://cloud.google.com/load-balancing/docs/negs/serverless-neg-concepts). 

Additionally a Domain Name is linked to a reserved static IP address and joined to the External Balancer providing a signed SSL Certificate. The load balancer and SSL is then bound to the SNEGs and in turn registered with all GKE Clusters. 

This provides a multi-region load balanced set of GKE Clusters. In this demo all clusters run identical worloads configured using [Helm Charts](https://helm.sh/) which are also deployed with the [Pulumi Kubernetes/Helm provider](https://www.pulumi.com/registry/packages/kubernetes/api-docs/helm/). 

The GKE Clusters are additionally configured with [Managed Istio (Anthos Service Mesh)](https://istio.io/latest/about/service-mesh/) to give visibility of the workloads across a multi-cluster mesh. To enable this the clusters are also enrolled into [GKE Fleet Management](https://cloud.google.com/kubernetes-engine/docs/fleets-overview). 

## The Design

### High Level Design

![Google Cloud High Level Infrastructure Diagram](https://github.com/timbohiatt/gke-at-scale-pulumi/blob/thiatt/ft_readme/docs/001-google-cloud-infra.png?raw=true)

### Load Balancer Breakdown
When deploying a Google Cloud load balancer there is lots of configuration to consider. In our deployment we will be deploying a Layer 7 External Load Balancer. This loa balanacer breaks down into several components illustrated in the diagram below. 

![Google Cloud Design - Load Balancer Breakdown](https://github.com/timbohiatt/gke-at-scale-pulumi/blob/thiatt/ft_readme/docs/002-load-balancer-breakdown.png?raw=true)

1. External statis IP Address; Reserved and assigned as the entry to point to the global load balancer. This staitc IP address is assigned to your DNS Provider for validation for the SSL Certificate. 
2. Forwaring Rule; Responsible for forwarding TCP traffic that enters the Google Cloud Network via the static IP address to a given HTTP or HTTPS Target Proxy. In our deployment we deploy two forwarding Rules.
    - 1x HTTPS TCP traffic on 443 
    - 1x HTTP TCP traffic on port 80
3. Target Proxies; Responsible for linking the SSL certificates with a URL map tha detemrines how traffic will be routed through the Load Balancer to Google Cloud Compute backends. 
4. URL Map; Reads the L7 TCP headers and compares them against the URL Maps associated with the Target Proxies. The URL map contains the configuration that determines which URL domains and paths will route to which Google Compute Backend services (VM's, Serverless, GKE). In our deployment we configure a single URL map for our chosen domain and all traffic routed into this URL Map from the Target Proxies will be routed to our GKE Backend Services.
5. Backend Services are the bridge between the Load Balancer and the target compute; In our deployment our traffic will be routed from the URL map to a single Backend Service which will contain as series GKE Pod IP addresses (Network Endpoints) which will exist in a NEG (Network Endpoint Group).

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

    ```bash
    $ pulumi update
	Previewing update (dev):

        Type                                  Name                                          Plan      
    +   pulumi:pulumi:Stack                   gke-at-scale-dev                              create    
    +   ├─ gcp:projects:Service               gas-project-service-compute.googleapis.com    create     
    +   ├─ gcp:projects:Service               gas-project-service-container.googleapis.com  create     
    +   ├─ gcp:projects:IAMCustomRole         gas-iam-custom-role-autoneg                   create     
    +   ├─ gcp:iam:WorkloadIdentityPool       gas-wip-gke-cluster                           create     
    +   ├─ gcp:serviceAccount:Account         gas-service-account                           create     
    +   ├─ gcp:serviceAccount:Account         gas-service-account-autoneg                   create     
    +   ├─ gcp:compute:GlobalAddress          gas-glb-ip-address                            create     
    +   ├─ gcp:compute:ManagedSslCertificate  gas-glb-ssl-cert                              create     
    +   ├─ gcp:compute:Network                gas-vpc                                       create     
    +   ├─ gcp:compute:HealthCheck            gas-glb-tcp-health-check                      create     
    +   ├─ gcp:projects:IAMBinding            gas-iam-custom-role-binding-autoneg           create     
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-us-central1                create     
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-asia-east1                 create     
    +   ├─ gcp:compute:Firewall               gas-fw-ingress-allow-cluster-app-access       create     
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-europe-west6               create     
    +   ├─ gcp:compute:Firewall               gas-fw-ingress-allow-health-checks            create     
    +   ├─ gcp:container:Cluster              gas-gke-cluster-us-central1                   create     
    +   │  └─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-app-team-us-central1          create     
    +   ├─ gcp:compute:BackendService         gas-glb-bes                                   create     
    +   ├─ gcp:container:Cluster              gas-gke-cluster-asia-east1                    create     
    +   │  └─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-app-team-asia-east1           create     
    +   ├─ gcp:container:Cluster              gas-gke-cluster-europe-west6                  create     
    +   │  └─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-app-team-europe-west6         create     
    +   ├─ gcp:compute:URLMap                 gas-glb-url-map-http                          create     
    +   ├─ gcp:compute:URLMap                 gas-glb-url-map-https                         create     
    +   ├─ gcp:compute:TargetHttpsProxy       gas-glb-https-proxy                           create     
    +   ├─ gcp:compute:TargetHttpProxy        gas-glb-http-proxy                            create     
    +   ├─ gcp:compute:GlobalForwardingRule   gas-glb-https-forwarding-rule                 create     
    +   ├─ gcp:compute:GlobalForwardingRule   gas-glb-http-forwarding-rule                  create     
    +   ├─ gcp:container:NodePool             gas-gke-asia-east1-np-01                      create     
    +   │  ├─ gcp:serviceAccount:IAMBinding   gas-iam-svc-account-k8s-binding-asia-east1    create     
    +   │  ├─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-istiod-asia-east1       create     
    +   │  ├─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-cluster-ops-asia-east1        create     
    +   │  └─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-igw-asia-east1          create     
    +   ├─ gcp:container:NodePool             gas-gke-us-central1-np-01                     create     
    +   │  ├─ gcp:serviceAccount:IAMBinding   gas-iam-svc-account-k8s-binding-us-central1   create     
    +   │  ├─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-istiod-us-central1      create     
    +   │  ├─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-cluster-ops-us-central1       create     
    +   │  └─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-igw-us-central1         create     
    +   ├─ gcp:container:NodePool             gas-gke-europe-west6-np-01                    create     
    +   │  ├─ gcp:serviceAccount:IAMBinding   gas-iam-svc-account-k8s-binding-europe-west6  create     
    +   │  ├─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-istiod-europe-west6     create     
    +   │  ├─ kubernetes:helm.sh/v3:Chart     gas-helm-deploy-cluster-ops-europe-west6      create     
    +   │  └─ kubernetes:helm.sh/v3:Release   gas-helm-deploy-istio-igw-europe-west6        create     
    +   ├─ pulumi:providers:kubernetes        gas-gke-cluster-asia-east1-kubeconfig         create     
    +   ├─ pulumi:providers:kubernetes        gas-gke-cluster-us-central1-kubeconfig        create     
    +   ├─ pulumi:providers:kubernetes        gas-gke-cluster-europe-west6-kubeconfig       create     
    +   ├─ kubernetes:helm.sh/v3:Release      gas-helm-deploy-istio-base-asia-east1         create     
    +   ├─ kubernetes:helm.sh/v3:Release      gas-helm-deploy-istio-base-europe-west6       create     
    +   ├─ kubernetes:helm.sh/v3:Release      gas-helm-deploy-istio-base-us-central1        create     
    +   ├─ kubernetes:core/v1:Namespace       gas-k8s-namespace-app-us-central1             create     
    +   ├─ kubernetes:core/v1:Namespace       gas-k8s-namespace-app-europe-west6            create     
    +   └─ kubernetes:core/v1:Namespace       gas-k8s-namespace-app-asia-east1              create   

	Resources:
        + 54 to create

	Updating (dev):

        Type                                  Name                                          Status              Info
    +   pulumi:pulumi:Stack                   gke-at-scale-dev                              creating (486s).    [GAS INFO] - Cloud Region: europe-central2 - SKIPPING
    +   ├─ gcp:projects:Service               gas-project-service-compute.googleapis.com    created (95s)       
    +   ├─ gcp:projects:Service               gas-project-service-container.googleapis.com  created (95s)       
    +   ├─ gcp:serviceAccount:Account         gas-service-account-autoneg                   created (1s)        
    +   ├─ gcp:projects:IAMCustomRole         gas-iam-custom-role-autoneg                   created (2s)        
    +   ├─ gcp:serviceAccount:Account         gas-service-account                           created (2s)        
    +   ├─ gcp:iam:WorkloadIdentityPool       gas-wip-gke-cluster                           created (12s)       
    +   ├─ gcp:projects:IAMBinding            gas-iam-custom-role-binding-autoneg           created (8s)        
    +   ├─ gcp:compute:HealthCheck            gas-glb-tcp-health-check                      created (12s)       
    +   ├─ gcp:compute:ManagedSslCertificate  gas-glb-ssl-cert                              created (12s)       
    +   ├─ gcp:compute:GlobalAddress          gas-glb-ip-address                            created (12s)       
    +   ├─ gcp:compute:Network                gas-vpc                                       created (12s)       
    +   ├─ gcp:compute:BackendService         gas-glb-bes                                   created (21s)       
    +   ├─ gcp:compute:Firewall               gas-fw-ingress-allow-health-checks            created (11s)       
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-us-central1                created (13s)       
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-asia-east1                 created (31s)       
    +   ├─ gcp:compute:Firewall               gas-fw-ingress-allow-cluster-app-access       created (12s)       
    +   ├─ gcp:compute:Subnetwork             gas-vpc-subnetwork-europe-west6               created (24s)       
    +   ├─ gcp:container:Cluster              gas-gke-cluster-us-central1                   creating (361s)..   
    +   ├─ gcp:compute:URLMap                 gas-glb-url-map-http                          created (11s)       
    +   ├─ gcp:compute:URLMap                 gas-glb-url-map-https                         created (12s)       
    +   ├─ gcp:container:Cluster              gas-gke-cluster-europe-west6                  creating (351s)     
    +   ├─ gcp:container:Cluster              gas-gke-cluster-asia-east1                    creating (344s)...  
    +   ├─ gcp:compute:TargetHttpProxy        gas-glb-http-proxy                            created (11s)       
    +   ├─ gcp:compute:TargetHttpsProxy       gas-glb-https-proxy                           created (11s)       
    +   ├─ gcp:compute:GlobalForwardingRule   gas-glb-http-forwarding-rule                  created (18s)       
    +   └─ gcp:compute:GlobalForwardingRule   gas-glb-https-forwarding-rule                 created (16s)       

    Outputs:
        ClusterName: "helloworld-9b9530f"
        KubeConfig : "<KUBECONFIG_CONTENTS>"

	Resources:
        + 2 created

    Duration: 3m3s
    ```

1. Once you've finished experimenting, tear down your stack's resources by destroying and removing it:

    ```bash
    $ pulumi destroy --yes
    $ pulumi stack rm --yes
    ```