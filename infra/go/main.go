package main

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/container"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/iam"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
	gkehub "github.com/pulumi/pulumi-google-native/sdk/go/google/gkehub/v1alpha"

	k8s "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	//k8sCorev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type CloudRegion struct {
	Enabled              bool
	Region               string
	SubnetIp             string
	GKECluster           *container.Cluster
	GKEClusterKubeconfig pulumi.StringOutput
	KubernetesProvider   *k8s.Provider
	//Subnet          *compute.Subnetwork
	//CloudRunService *cloudrunv2.Service
}

var CloudRegions = []CloudRegion{
	CloudRegion{
		Enabled:  true,
		Region:   "us-central1",
		SubnetIp: "10.128.50.0/24",
	},
	CloudRegion{
		Enabled:  true,
		Region:   "europe-west6",
		SubnetIp: "10.128.100.0/24",
	},
	CloudRegion{
		Enabled:  true,
		Region:   "asia-east1",
		SubnetIp: "10.128.150.0/24",
	},
	CloudRegion{
		Enabled:  true,
		Region:   "australia-southeast1",
		SubnetIp: "10.128.200.0/24",
	},
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		URNPrefix := "gas"
		//Domain := "can-scale.com"
		GCPProjectId := "thiatt-manual-005"
		GCPGKEClusterName := fmt.Sprintf("%s-gke-cluster", URNPrefix)

		// Declare an Array of API's To Enable.
		var GCPServices = []string{
			"artifactregistry.googleapis.com",
			"compute.googleapis.com",
			"container.googleapis.com",
			"mesh.googleapis.com",
			"anthos.googleapis.com",
		}

		// Create a Pulumi Resource Array Object to Store Specific Dependancies within.
		GCPDependencies := []pulumi.Resource{}

		URN := fmt.Sprintf("%s-project", URNPrefix)
		GCPProject, err := organizations.LookupProject(ctx, &organizations.LookupProjectArgs{
			ProjectId: &GCPProjectId,
		})
		if err != nil {
			return err
		}

		// Enable all the Required Google API's on the Specified Project.
		for _, Service := range GCPServices {
			URN := fmt.Sprintf("%s-project-service-%s", URNPrefix, Service)
			GCPService, err := projects.NewService(ctx, URN, &projects.ServiceArgs{
				DisableDependentServices: pulumi.Bool(true),
				Project:                  pulumi.String(GCPProjectId),
				Service:                  pulumi.String(Service),
				DisableOnDestroy:         pulumi.Bool(false),
			})
			if err != nil {
				return err
			}
			// Append API Enablement Resources to a Depenancies Array
			GCPDependencies = append(GCPDependencies, GCPService)
		}

		// Enable Anthos Service Mesh Fleets
		URN = fmt.Sprintf("%s-local-cmd-gcloud-enable-fleets", URNPrefix)
		CMDLocalGcloudEnableFleets, err := local.NewCommand(ctx, URN, &local.CommandArgs{
			Create: pulumi.Sprintf("gcloud container fleet mesh enable --project %s", GCPProjectId),
			Update: pulumi.Sprintf("gcloud container fleet mesh enable --project %s", GCPProjectId),
			Delete: pulumi.Sprintf("gcloud container fleet mesh disable --project %s", GCPProjectId),
		}, pulumi.DependsOn(GCPDependencies))
		if err != nil {
			return err
		}

		// Append Mesh Feature Enablement to a Depenancies Array
		GCPDependencies = append(GCPDependencies, CMDLocalGcloudEnableFleets)

		// Create Google Cloud Workload Identity Pool for GKE
		URN = fmt.Sprintf("%s-wip-gke-cluster", URNPrefix)
		GCPWorkloadIdentityPoolGKE, err := iam.NewWorkloadIdentityPool(ctx, URN, &iam.WorkloadIdentityPoolArgs{
			Project:                pulumi.String(GCPProjectId),
			Description:            pulumi.String("GKE at Scale - Workload Identity Pool for GKE Cluster"),
			Disabled:               pulumi.Bool(false),
			DisplayName:            pulumi.String(URN),
			WorkloadIdentityPoolId: pulumi.String(fmt.Sprintf("%s-wip-gke-008", URNPrefix)),
		}, pulumi.DependsOn(GCPDependencies))
		if err != nil {
			return err
		}
		// Export Google Cloud Workload Identity Pool
		ctx.Export(URN, GCPWorkloadIdentityPoolGKE)

		// Append Workload Identity Pool to a Depenancies Array
		GCPDependencies = append(GCPDependencies, GCPWorkloadIdentityPoolGKE)

		// Create Google Cloud VPC Network
		URN = fmt.Sprintf("%s-vpc", URNPrefix)
		GCPNetwork, err := compute.NewNetwork(ctx, URN, &compute.NetworkArgs{
			Project:               pulumi.String(GCPProjectId),
			Name:                  pulumi.String(URN),
			Description:           pulumi.String("GKE at Scale - Global VPC Network"),
			AutoCreateSubnetworks: pulumi.Bool(false),
		}, pulumi.DependsOn(GCPDependencies))
		if err != nil {
			return err
		}

		// Export Google Cloud VPC Network
		ctx.Export(URN, GCPNetwork)

		// Create Google Cloud Service Account
		URN = fmt.Sprintf("%s-service-account", URNPrefix)
		GCPServiceAccount, err := serviceaccount.NewAccount(ctx, URN, &serviceaccount.AccountArgs{
			Project:     pulumi.String(GCPProjectId),
			AccountId:   pulumi.String("svc-gke-at-scale-admin"),
			DisplayName: pulumi.String("GKE at Scale - Admin Service Account"),
		})
		if err != nil {
			return err
		}
		// Export Google Cloud Service Account
		ctx.Export(URN, GCPServiceAccount)

		// Create Artifact Registry Repository
		URN = fmt.Sprintf("%s-artifact-registry-repository", URNPrefix)
		GCPArtifactRegistryRepo, err := artifactregistry.NewRepository(ctx, URN, &artifactregistry.RepositoryArgs{
			Project:      pulumi.String(GCPProjectId),
			Description:  pulumi.String("GKE at Scale"),
			Format:       pulumi.String("DOCKER"),
			Location:     pulumi.String("europe"),
			RepositoryId: pulumi.String("gke-at-scale"),
		}, pulumi.DependsOn(GCPDependencies)) // Ensure API Enablement Dependency here;
		if err != nil {
			return err
		}

		GCPDependencies = append(GCPDependencies, GCPArtifactRegistryRepo)

		// Export Artifact Registry Repository
		ctx.Export(URN, GCPArtifactRegistryRepo)

		// Create GKE Hub Fleet
		URN = fmt.Sprintf("%s-gke-fleet", URNPrefix)
		GKEFleet, err := gkehub.NewFleet(ctx, URN, &gkehub.FleetArgs{
			Project:     pulumi.String(GCPProjectId),
			DisplayName: pulumi.String(fmt.Sprintf("%s-gke-cluster", URNPrefix)),
			Location:    pulumi.String("global"),
		}, pulumi.DependsOn(GCPDependencies))

		GCPDependencies = append(GCPDependencies, GKEFleet)

		// Export GKE Fleet
		ctx.Export(URN, GKEFleet)

		// Process Each Cloud Region;
		for _, CloudRegion := range CloudRegions {
			if CloudRegion.Enabled {

				// Create VPC Subnet for Cloud Region
				URN := fmt.Sprintf("%s-vpc-subnetwork-%s", URNPrefix, CloudRegion.Region)
				GCPSubnetwork, err := compute.NewSubnetwork(ctx, URN, &compute.SubnetworkArgs{
					Project:               pulumi.String(GCPProjectId),
					Name:                  pulumi.String(URN),
					Description:           pulumi.String(fmt.Sprintf("GKE at Scale - VPC Subnet - %s", CloudRegion.Region)),
					IpCidrRange:           pulumi.String(CloudRegion.SubnetIp),
					Region:                pulumi.String(CloudRegion.Region),
					Network:               GCPNetwork.ID(),
					PrivateIpGoogleAccess: pulumi.Bool(true),
				})
				if err != nil {
					return err
				}
				// Export Service Account
				ctx.Export(URN, GCPSubnetwork)

				// Create GKE Autopilot Cluster for Cloud Region
				URN = fmt.Sprintf("%s-gke-cluster-%s", URNPrefix, CloudRegion.Region)
				GCPGKECluster, err := container.NewCluster(ctx, URN, &container.ClusterArgs{
					Project:         pulumi.String(GCPProjectId),
					Name:            pulumi.String(GCPGKEClusterName),
					Network:         GCPNetwork.ID(),
					Subnetwork:      GCPSubnetwork.ID(),
					Location:        pulumi.String(CloudRegion.Region),
					EnableAutopilot: pulumi.Bool(true),
					VerticalPodAutoscaling: &container.ClusterVerticalPodAutoscalingArgs{
						Enabled: pulumi.Bool(true),
					},
					IpAllocationPolicy: &container.ClusterIpAllocationPolicyArgs{},
					MasterAuthorizedNetworksConfig: &container.ClusterMasterAuthorizedNetworksConfigArgs{
						CidrBlocks: &container.ClusterMasterAuthorizedNetworksConfigCidrBlockArray{
							&container.ClusterMasterAuthorizedNetworksConfigCidrBlockArgs{
								CidrBlock:   pulumi.String("0.0.0.0/0"),
								DisplayName: pulumi.String("Global Public Access"),
							},
						},
					},
				}, pulumi.DependsOn(GCPDependencies))
				if err != nil {
					return err
				}
				// Store GCP GKE Cluster
				CloudRegion.GKECluster = GCPGKECluster

				// Add Cluster as a Explicit Dependency.
				GCPDependencies = append(GCPDependencies, GCPGKECluster)

				// Export GKE Autopilot Cluster
				ctx.Export(URN, GCPGKECluster)

				// Build Kubeconfig for accessing the cluster
				CloudRegion.GKEClusterKubeconfig = pulumi.Sprintf(`apiVersion: v1
				clusters:
				- cluster:
					certificate-authority-data: %[3]s
					server: https://%[2]s
				name: %[1]s
				contexts:
				- context:
					cluster: %[1]s
					user: %[1]s
				name: %[1]s
				current-context: %[1]s
				kind: Config
				preferences: {}
				users:
				- name: %[1]s
				user:
					exec:
					apiVersion: client.authentication.k8s.io/v1beta1
					command: gke-gcloud-auth-plugin
					installHint: Install gke-gcloud-auth-plugin for use with kubectl by following
						https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke
					provideClusterInfo: true
			`, CloudRegion.GKECluster.Name, CloudRegion.GKECluster.Endpoint, CloudRegion.GKECluster.MasterAuth.ClusterCaCertificate().Elem())

				// Export Kubeconfig
				URN = fmt.Sprintf("%s-gke-cluster-kubeconfig-%s", URNPrefix, CloudRegion.Region)
				ctx.Export(URN, CloudRegion.GKEClusterKubeconfig)
			}
		}

		// Configure Google Cloud Kubernetes & Clusters;
		for idx, CloudRegion := range CloudRegions {
			if CloudRegion.Enabled {

				URN = fmt.Sprintf("%s-local-cmd-kubctl-get-ns-%s", URNPrefix, CloudRegion.Region)
				GKEConfig, err := local.NewCommand(ctx, URN, &local.CommandArgs{
					Create: pulumi.Sprintf("./gke-config/setup.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", URNPrefix), CloudRegion.Region, GCPProjectId, GCPProject.Number, idx),
					Update: pulumi.Sprintf("./gke-config/setup.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", URNPrefix), CloudRegion.Region, GCPProjectId, GCPProject.Number, idx),
					Delete: pulumi.Sprintf("./gke-config/delete.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", URNPrefix), CloudRegion.Region, GCPProjectId, GCPProject.Number, idx),
				}, pulumi.DependsOn(GCPDependencies))
				if err != nil {
					return err
				}

				// Add Cluster as a Explicit Dependency.
				GCPDependencies = append(GCPDependencies, GKEConfig)
				ctx.Export(URN, GKEConfig)
			}
		}

		// Configure Multi Cluster ASM Mesh;
		cmd := fmt.Sprintf("./gke-config/asmcli create-mesh %s", GCPProjectId)
		for _, CloudRegion := range CloudRegions {
			if CloudRegion.Enabled {
				// ${PROJECT_1}/${LOCATION_1}/${CLUSTER_1}
				cmd = fmt.Sprintf("%s %s/%s/%s", cmd, GCPProjectId, CloudRegion.Region, GCPGKEClusterName)
			}
		}

		URN = fmt.Sprintf("%s-local-cmd-ams-multicluster-mesh", URNPrefix)
		GCPASMMesh, err := local.NewCommand(ctx, URN, &local.CommandArgs{
			Create: pulumi.String(cmd),
			Update: pulumi.String(cmd),
		}, pulumi.DependsOn(GCPDependencies))
		if err != nil {
			return err
		}
		// Add GKE ASM Mesh as a Explicit Dependency.
		GCPDependencies = append(GCPDependencies, GCPASMMesh)
		ctx.Export(URN, GCPASMMesh)



		// Construct Google Cloud Load Balancer


		return nil
	})
}
