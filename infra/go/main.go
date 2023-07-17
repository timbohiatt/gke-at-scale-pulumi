package main

import (
	"fmt"

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

	helm "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
)

type cloudRegion struct {
	Enabled            bool
	Region             string
	SubnetIp           string
	GKECluster         *container.Cluster
	KubernetesProvider *k8s.Provider
	//Subnet          *compute.Subnetwork
	//CloudRunService *cloudrunv2.Service
}

var CloudRegions = []cloudRegion{
	cloudRegion{
		Enabled:  false,
		Region:   "us-central1",
		SubnetIp: "10.128.50.0/24",
	},
	cloudRegion{
		Enabled:  true,
		Region:   "europe-west6",
		SubnetIp: "10.128.100.0/24",
	},
	cloudRegion{
		Enabled:  true,
		Region:   "asia-east1",
		SubnetIp: "10.128.150.0/24",
	},
	cloudRegion{
		Enabled:  false,
		Region:   "australia-southeast1",
		SubnetIp: "10.128.200.0/24",
	},
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		urnPrefix := "gas"
		domain := "gke-at-scale.com"
		gcpProjectId := "thiatt-manual-011"
		gcpGKEClusterName := fmt.Sprintf("%s-gke-cluster", urnPrefix)

		// Create a Pulumi Resource Array Object to Store Specific Dependancies within.
		gcpDependencies := []pulumi.Resource{}

		/* Google Cloud Project Service Enablement */

		// Declare an Array of API's To Enable.
		var GCPServices = []string{
			"artifactregistry.googleapis.com",
			"compute.googleapis.com",
			"container.googleapis.com",
			"mesh.googleapis.com",
			"anthos.googleapis.com",
		}

		// Look up Existing Google Cloud Project
		urn := fmt.Sprintf("%s-project", urnPrefix)
		gcpProject, err := organizations.LookupProject(ctx, &organizations.LookupProjectArgs{
			ProjectId: &gcpProjectId,
		})
		if err != nil {
			return err
		}

		// Enable Google API's on the Specified Project.
		for _, Service := range GCPServices {
			urn := fmt.Sprintf("%s-project-service-%s", urnPrefix, Service)
			gcpService, err := projects.NewService(ctx, urn, &projects.ServiceArgs{
				DisableDependentServices: pulumi.Bool(true),
				Project:                  pulumi.String(gcpProjectId),
				Service:                  pulumi.String(Service),
				DisableOnDestroy:         pulumi.Bool(false),
			})
			if err != nil {
				return err
			}
			// Append API Enablement Resources to a Depenancies Array
			gcpDependencies = append(gcpDependencies, gcpService)
		}

		// Create Google Cloud Service Account
		urn = fmt.Sprintf("%s-service-account", urnPrefix)
		_, err = serviceaccount.NewAccount(ctx, urn, &serviceaccount.AccountArgs{
			Project:     pulumi.String(gcpProjectId),
			AccountId:   pulumi.String("svc-gke-at-scale-admin"),
			DisplayName: pulumi.String("GKE at Scale - Admin Service Account"),
		})
		if err != nil {
			return err
		}

		// Create AutoNeg Service Account
		urn = fmt.Sprintf("%s-service-account-autoneg", urnPrefix)
		gcpServiceAccountAutoNeg, err := serviceaccount.NewAccount(ctx, urn, &serviceaccount.AccountArgs{
			Project:     pulumi.String(gcpProjectId),
			AccountId:   pulumi.String("autoneg-system"),
			DisplayName: pulumi.String("autoneg"),
		})
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-iam-custom-role-autoneg", urnPrefix)
		gcpCustomIAMRoleAutoNeg, err := projects.NewIAMCustomRole(ctx, urn, &projects.IAMCustomRoleArgs{
			Project:     pulumi.String(gcpProjectId),
			Description: pulumi.String("Custom IAM Role - GKE AutoNeg"),
			Permissions: pulumi.StringArray{
				pulumi.String("compute.backendServices.get"),
				pulumi.String("compute.backendServices.update"),
				pulumi.String("compute.regionBackendServices.get"),
				pulumi.String("compute.regionBackendServices.update"),
				pulumi.String("compute.networkEndpointGroups.use"),
				pulumi.String("compute.healthChecks.useReadOnly"),
				pulumi.String("compute.regionHealthChecks.useReadOnly"),
			},

			RoleId: pulumi.String("autoneg"),
			Title:  pulumi.String("GKE AutoNeg"),
		}, pulumi.DependsOn([]pulumi.Resource{gcpServiceAccountAutoNeg}))
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-iam-custom-role-binding-autoneg", urnPrefix)
		_, err = projects.NewIAMBinding(ctx, urn, &projects.IAMBindingArgs{
			Members: pulumi.StringArray{
				pulumi.String(fmt.Sprintf("serviceAccount:autoneg-system@%s.iam.gserviceaccount.com", gcpProjectId)),
			},
			Project: pulumi.String(gcpProjectId),
			Role:    gcpCustomIAMRoleAutoNeg.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{gcpServiceAccountAutoNeg, gcpCustomIAMRoleAutoNeg}))
		if err != nil {
			return err
		}

		// **** TODO: Find Pulumi Option? ****
		// Enable Anthos Service Mesh Fleets
		urn = fmt.Sprintf("%s-local-cmd-gcloud-enable-fleets", urnPrefix)
		cmdLocalGcloudEnableFleets, err := local.NewCommand(ctx, urn, &local.CommandArgs{
			Create: pulumi.Sprintf("gcloud container fleet mesh enable --project %s", gcpProjectId),
			Update: pulumi.Sprintf("gcloud container fleet mesh enable --project %s", gcpProjectId),
			Delete: pulumi.Sprintf("gcloud container fleet mesh disable --project %s", gcpProjectId),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}

		// Append Mesh Feature Enablement to a Depenancies Array
		gcpDependencies = append(gcpDependencies, cmdLocalGcloudEnableFleets)

		// Create Google Cloud Workload Identity Pool for GKE
		urn = fmt.Sprintf("%s-wip-gke-cluster", urnPrefix)
		gcpWorkloadIdentityPoolGKE, err := iam.NewWorkloadIdentityPool(ctx, urn, &iam.WorkloadIdentityPoolArgs{
			Project:                pulumi.String(gcpProjectId),
			Description:            pulumi.String("GKE at Scale - Workload Identity Pool for GKE Cluster"),
			Disabled:               pulumi.Bool(false),
			DisplayName:            pulumi.String(urn),
			WorkloadIdentityPoolId: pulumi.String(fmt.Sprintf("%s-wip-gke-014", urnPrefix)), // **** TODO: Replace with Pulumi RANDOM ID? ****
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}
		// Append Workload Identity Pool to a Depenancies Array
		gcpDependencies = append(gcpDependencies, gcpWorkloadIdentityPoolGKE)

		/* Google Cloud Project Network Configuration */

		// Create Google Cloud VPC Network
		urn = fmt.Sprintf("%s-vpc", urnPrefix)
		gcpNetwork, err := compute.NewNetwork(ctx, urn, &compute.NetworkArgs{
			Project:               pulumi.String(gcpProjectId),
			Name:                  pulumi.String(urn),
			Description:           pulumi.String("GKE at Scale - Global VPC Network"),
			AutoCreateSubnetworks: pulumi.Bool(false),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}

		// Construct Google Cloud Load Balancer

		// Create Global Load Balancer Static IP Address
		urn = fmt.Sprintf("%s-glb-ip-address", urnPrefix)
		gcpGlobalAddress, err := compute.NewGlobalAddress(ctx, urn, &compute.GlobalAddressArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-glb-ip-address", urnPrefix)),
			AddressType: pulumi.String("EXTERNAL"),
			IpVersion:   pulumi.String("IPV4"),
			Description: pulumi.String("GKE At Scale - Global Load Balancer Static IP Address"),
		})
		if err != nil {
			return err
		}
		ctx.Export(urn, gcpGlobalAddress)

		// Create Managed SSL Certificate
		urn = fmt.Sprintf("%s-glb-ssl-cert", urnPrefix)
		gcpGLBManagedSSLCert, err := compute.NewManagedSslCertificate(ctx, urn, &compute.ManagedSslCertificateArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-glb-ssl-cert", urnPrefix)),
			Description: pulumi.String("Global Load Balancer - Managed SSL Certificate - GKE at Scale!"),
			Type:        pulumi.String("MANAGED"),
			Managed: &compute.ManagedSslCertificateManagedArgs{
				Domains: pulumi.StringArray{
					pulumi.String(domain),
				},
			},
		})

		var backendServiceBackendArray = compute.BackendServiceBackendArray{}
		urn = fmt.Sprintf("%s-glb-bes", urnPrefix)
		gcpBackendService, err := compute.NewBackendService(ctx, urn, &compute.BackendServiceArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-bes", urnPrefix)),
			Description: pulumi.String("Global Load Balancer - Backend Service - GKE At Scale!"),
			CdnPolicy: &compute.BackendServiceCdnPolicyArgs{
				ClientTtl:  pulumi.Int(5),
				DefaultTtl: pulumi.Int(5),
				MaxTtl:     pulumi.Int(5),
			},
			ConnectionDrainingTimeoutSec: pulumi.Int(10),
			Backends:                     backendServiceBackendArray,
		})
		if err != nil {
			return err
		}

		// Create URL Map
		urn = fmt.Sprintf("%s-glb-url-map-https", urnPrefix)
		gcpGLBURLMapHTTPS, err := compute.NewURLMap(ctx, urn, &compute.URLMapArgs{
			Project:        pulumi.String(gcpProjectId),
			Name:           pulumi.String(fmt.Sprintf("%s-glb-urlmap-https", urnPrefix)),
			Description:    pulumi.String("Global Load Balancer - HTTPS URL Map - GKE At Scale!"),
			DefaultService: gcpBackendService.SelfLink,
		})
		if err != nil {
			return err
		}

		// Create URL Map
		urn = fmt.Sprintf("%s-glb-url-map-http", urnPrefix)
		gcpGLBURLMapHTTP, err := compute.NewURLMap(ctx, urn, &compute.URLMapArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-glb-urlmap-http", urnPrefix)),
			Description: pulumi.String("Global Load Balancer - HTTP URL Map - GKE At Scale!"),
			HostRules: &compute.URLMapHostRuleArray{
				&compute.URLMapHostRuleArgs{
					Hosts: pulumi.StringArray{
						pulumi.String(domain),
					},
					PathMatcher: pulumi.String("all-paths"),
					Description: pulumi.String("Default Route All Paths"),
				},
			},
			PathMatchers: &compute.URLMapPathMatcherArray{
				&compute.URLMapPathMatcherArgs{
					Name:           pulumi.String("all-paths"),
					DefaultService: gcpBackendService.SelfLink,
					PathRules: &compute.URLMapPathMatcherPathRuleArray{
						&compute.URLMapPathMatcherPathRuleArgs{
							Paths: pulumi.StringArray{
								pulumi.String("/*"),
							},
							UrlRedirect: &compute.URLMapPathMatcherPathRuleUrlRedirectArgs{
								StripQuery:    pulumi.Bool(false),
								HttpsRedirect: pulumi.Bool(true),
							},
						},
					},
				},
			},
			DefaultService: gcpBackendService.SelfLink,
		})
		if err != nil {
			return err
		}

		// Create Target HTTPS Proxy
		urn = fmt.Sprintf("%s-glb-https-proxy", urnPrefix)
		gcpGLBTargetHTTPSProxy, err := compute.NewTargetHttpsProxy(ctx, urn, &compute.TargetHttpsProxyArgs{
			Project: pulumi.String(gcpProjectId),
			Name:    pulumi.String(urn),
			UrlMap:  gcpGLBURLMapHTTPS.SelfLink,
			SslCertificates: pulumi.StringArray{
				gcpGLBManagedSSLCert.SelfLink,
			},
		})
		if err != nil {
			return err
		}

		// Create Target HTTP Proxy
		urn = fmt.Sprintf("%s-glb-http-proxy", urnPrefix)
		gcpGLBTargetHTTPProxy, err := compute.NewTargetHttpProxy(ctx, urn, &compute.TargetHttpProxyArgs{
			Project: pulumi.String(gcpProjectId),
			Name:    pulumi.String(urn),
			UrlMap:  gcpGLBURLMapHTTP.SelfLink,
		})
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-glb-https-forwarding-rule", urnPrefix)
		_, err = compute.NewGlobalForwardingRule(ctx, urn, &compute.GlobalForwardingRuleArgs{
			Project:             pulumi.String(gcpProjectId),
			Target:              gcpGLBTargetHTTPSProxy.SelfLink,
			IpAddress:           gcpGlobalAddress.SelfLink,
			PortRange:           pulumi.String("443"),
			LoadBalancingScheme: pulumi.String("EXTERNAL"),
		})
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-glb-http-forwarding-rule", urnPrefix)
		_, err = compute.NewGlobalForwardingRule(ctx, urn, &compute.GlobalForwardingRuleArgs{
			Project:             pulumi.String(gcpProjectId),
			Target:              gcpGLBTargetHTTPProxy.SelfLink,
			IpAddress:           gcpGlobalAddress.SelfLink,
			PortRange:           pulumi.String("80"),
			LoadBalancingScheme: pulumi.String("EXTERNAL"),
		})
		if err != nil {
			return err
		}

		// Create GKE Hub Fleet
		urn = fmt.Sprintf("%s-gke-fleet", urnPrefix)
		gkeFleet, err := gkehub.NewFleet(ctx, urn, &gkehub.FleetArgs{
			Project:     pulumi.String(gcpProjectId),
			DisplayName: pulumi.String(fmt.Sprintf("%s-gke-cluster", urnPrefix)),
			Location:    pulumi.String("global"),
		}, pulumi.DependsOn(gcpDependencies))
		gcpDependencies = append(gcpDependencies, gkeFleet)

		/* Configure Resources in Each Cloud Region */

		// Process Each Cloud Region;
		for _, cloudRegion := range CloudRegions {
			if !cloudRegion.Enabled {
				continue
			}

			// Create VPC Subnet for Cloud Region
			urn := fmt.Sprintf("%s-vpc-subnetwork-%s", urnPrefix, cloudRegion.Region)
			gcpSubnetwork, err := compute.NewSubnetwork(ctx, urn, &compute.SubnetworkArgs{
				Project:               pulumi.String(gcpProjectId),
				Name:                  pulumi.String(urn),
				Description:           pulumi.String(fmt.Sprintf("GKE at Scale - VPC Subnet - %s", cloudRegion.Region)),
				IpCidrRange:           pulumi.String(cloudRegion.SubnetIp),
				Region:                pulumi.String(cloudRegion.Region),
				Network:               gcpNetwork.ID(),
				PrivateIpGoogleAccess: pulumi.Bool(true),
			})
			if err != nil {
				return err
			}

			// Create GKE Autopilot Cluster for Cloud Region
			urn = fmt.Sprintf("%s-gke-cluster-%s", urnPrefix, cloudRegion.Region)
			gcpGKECluster, err := container.NewCluster(ctx, urn, &container.ClusterArgs{
				Project:         pulumi.String(gcpProjectId),
				Name:            pulumi.String(gcpGKEClusterName),
				Network:         gcpNetwork.ID(),
				Subnetwork:      gcpSubnetwork.ID(),
				Location:        pulumi.String(cloudRegion.Region),
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
			}, pulumi.DependsOn(gcpDependencies))
			if err != nil {
				return err
			}
			// Store GCP GKE Cluster
			cloudRegion.GKECluster = gcpGKECluster
			// Add Cluster as a Explicit Dependency.
			gcpDependencies = append(gcpDependencies, gcpGKECluster)
		}

		// Configure Google Cloud Kubernetes & Clusters;
		for idx, cloudRegion := range CloudRegions {
			if !cloudRegion.Enabled {
				continue
			}

			urn = fmt.Sprintf("%s-local-cmd-kubctl-get-ns-%s", urnPrefix, cloudRegion.Region)
			gkeConfig, err := local.NewCommand(ctx, urn, &local.CommandArgs{
				Create: pulumi.Sprintf("./gke-config/setup.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", urnPrefix), cloudRegion.Region, gcpProjectId, gcpProject.Number, idx),
				Update: pulumi.Sprintf("./gke-config/setup.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", urnPrefix), cloudRegion.Region, gcpProjectId, gcpProject.Number, idx),
				Delete: pulumi.Sprintf("./gke-config/delete.sh -c %s -r %s -p %s -n %s -l %d ", fmt.Sprintf("%s-gke-cluster", urnPrefix), cloudRegion.Region, gcpProjectId, gcpProject.Number, idx),
			}, pulumi.DependsOn(gcpDependencies))
			if err != nil {
				return err
			}
			// Add Cluster as a Explicit Dependency.
			gcpDependencies = append(gcpDependencies, gkeConfig)

			urn = fmt.Sprintf("%s-helm-deploy-cluster-ops-%s", urnPrefix, cloudRegion.Region)
			helmClusterOps, err := helm.NewChart(ctx, urn, helm.ChartArgs{
				Chart:   pulumi.String("cluster-ops"),
				Version: pulumi.String("0.1.1"),
				Path:    pulumi.String("../../apps/helm"),
				Values: pulumi.Map{
					"global": pulumi.Map{
						"labels": pulumi.Map{
							"region": pulumi.String(cloudRegion.Region),
						},
					},
					"app": pulumi.Map{
						"region": pulumi.String(cloudRegion.Region),
					},
					"autoneg": pulumi.Map{
						"serviceAccount": pulumi.Map{
							"annotations": pulumi.Map{
								"iam.gke.io/gcp-service-account": pulumi.String(fmt.Sprintf("autoneg-system@%s.iam.gserviceaccount.com", gcpProjectId)),
							}},
					},
				},
			}, pulumi.DependsOn(gcpDependencies))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-iam-svc-account-k8s-binding-%s", urnPrefix, cloudRegion.Region)
			_, err = serviceaccount.NewIAMBinding(ctx, urn, &serviceaccount.IAMBindingArgs{
				ServiceAccountId: gcpServiceAccountAutoNeg.Name,
				Role:             pulumi.String("roles/iam.workloadIdentityUser"),
				Members: pulumi.StringArray{
					pulumi.String(fmt.Sprintf("serviceAccount:%s.svc.id.goog[autoneg-system/autoneg-controller-manager]", gcpProjectId)),
				},
			}, pulumi.DependsOn([]pulumi.Resource{helmClusterOps}))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-app-team-1-%s", urnPrefix, cloudRegion.Region)
			_, err = helm.NewChart(ctx, urn, helm.ChartArgs{
				Chart:   pulumi.String("app-team-1"),
				Version: pulumi.String("0.1.1"),
				Path:    pulumi.String("../../apps/helm"),
				Values: pulumi.Map{
					"global": pulumi.Map{
						"labels": pulumi.Map{
							"region": pulumi.String(cloudRegion.Region),
						},
					},
				},
			}, pulumi.DependsOn(gcpDependencies))
			if err != nil {
				return err
			}

		}

		// Configure Multi Cluster ASM Mesh;
		cmd := fmt.Sprintf("./gke-config/asmcli create-mesh %s", gcpProjectId)
		for _, cloudRegion := range CloudRegions {
			if !cloudRegion.Enabled {
				continue
			}
			// Prepare Command Line Statement
			cmd = fmt.Sprintf("%s %s/%s/%s", cmd, gcpProjectId, cloudRegion.Region, gcpGKEClusterName)

		}

		urn = fmt.Sprintf("%s-local-cmd-ams-multicluster-mesh", urnPrefix)
		gcpASMMesh, err := local.NewCommand(ctx, urn, &local.CommandArgs{
			Create: pulumi.String(cmd),
			Update: pulumi.String(cmd),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}
		// Add GKE ASM Mesh as a Explicit Dependency.
		gcpDependencies = append(gcpDependencies, gcpASMMesh)
		ctx.Export(urn, gcpASMMesh)

		return nil
	})
}
