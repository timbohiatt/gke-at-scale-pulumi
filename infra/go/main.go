package main

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/container"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/iam"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/organizations"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	k8s "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helm "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type cloudRegion struct {
	Id             string
	Enabled        bool
	Region         string
	SubnetIp       string
	GKECluster     *container.Cluster
	GKEClusterName string
}

var CloudRegions = []cloudRegion{
	cloudRegion{
		Id:       "001",
		Enabled:  false,
		Region:   "us-central1",
		SubnetIp: "10.128.50.0/24",
	},
	cloudRegion{
		Id:       "002",
		Enabled:  true,
		Region:   "europe-west6",
		SubnetIp: "10.128.100.0/24",
	},
	cloudRegion{
		Id:       "003",
		Enabled:  false,
		Region:   "asia-east1",
		SubnetIp: "10.128.150.0/24",
	},
}

// Declare an Array of API's To Enable.
var GCPServices = []string{
	//"artifactregistry.googleapis.com",
	"compute.googleapis.com",
	"container.googleapis.com",
	//"mesh.googleapis.com",
	//"anthos.googleapis.com",
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		gcpDependencies := []pulumi.Resource{}

		cfg := config.New(ctx, "")
		urnPrefix := cfg.Require("prefix")
		domain := cfg.Require("domainName")
		gcpProjectId := cfg.Require("gcpProjectId")
		//gcpGKEClusterName := fmt.Sprintf("%s-gke-cluster", urnPrefix)

		// Look up Existing Google Cloud Project
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

		// Create Global Load Balancer Static IP Address
		urn := fmt.Sprintf("%s-glb-ip-address", urnPrefix)
		gcpGlobalAddress, err := compute.NewGlobalAddress(ctx, urn, &compute.GlobalAddressArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(urn),
			AddressType: pulumi.String("EXTERNAL"),
			IpVersion:   pulumi.String("IPV4"),
			Description: pulumi.String("GKE At Scale - Global Load Balancer - Static IP Address"),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}

		// Create Managed SSL Certificate
		urn = fmt.Sprintf("%s-glb-ssl-cert", urnPrefix)
		gcpGLBManagedSSLCert, err := compute.NewManagedSslCertificate(ctx, urn, &compute.ManagedSslCertificateArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(urn),
			Description: pulumi.String("GKE at Scale - Global Load Balancer - Managed SSL Certificate"),
			Type:        pulumi.String("MANAGED"),
			Managed: &compute.ManagedSslCertificateManagedArgs{
				Domains: pulumi.StringArray{
					pulumi.String(domain),
				},
			},
		}, pulumi.DependsOn(gcpDependencies))

		urn = fmt.Sprintf("%s-iam-custom-role-autoneg", urnPrefix)
		gcpIAMRoleAutoNeg, err := projects.NewIAMCustomRole(ctx, urn, &projects.IAMCustomRoleArgs{
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

			RoleId: pulumi.String(fmt.Sprintf("%s_iam_role_autoneg_system", urnPrefix)),
			Title:  pulumi.String("GKE at Scale - AutoNEG"),
		})
		if err != nil {
			return err
		}

		// Create Google Cloud Service Account
		urn = fmt.Sprintf("%s-service-account", urnPrefix)
		gcpServiceAccount, err := serviceaccount.NewAccount(ctx, urn, &serviceaccount.AccountArgs{
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
			DisplayName: pulumi.String("GKE at Scale - AutoNEG Service Account"),
		})
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-iam-custom-role-binding-autoneg", urnPrefix)
		_, err = projects.NewIAMBinding(ctx, urn, &projects.IAMBindingArgs{
			Members: pulumi.StringArray{
				pulumi.String(fmt.Sprintf("serviceAccount:autoneg-system@%s.iam.gserviceaccount.com", gcpProjectId)),
			},
			Project: pulumi.String(gcpProjectId),
			Role:    gcpIAMRoleAutoNeg.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{gcpServiceAccountAutoNeg}))
		if err != nil {
			return err
		}

		// Create Google Cloud Workload Identity Pool for GKE
		urn = fmt.Sprintf("%s-wip-gke-cluster", urnPrefix)
		_, err = iam.NewWorkloadIdentityPool(ctx, urn, &iam.WorkloadIdentityPoolArgs{
			Project:                pulumi.String(gcpProjectId),
			Description:            pulumi.String("GKE at Scale - Workload Identity Pool for GKE Cluster"),
			Disabled:               pulumi.Bool(false),
			DisplayName:            pulumi.String(urn),
			WorkloadIdentityPoolId: pulumi.String(fmt.Sprintf("%s-wip-gke-0018", urnPrefix)), // **** TODO: Replace with Pulumi RANDOM ID? ****
		})
		if err != nil {
			return err
		}

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

		// Create Firewall Rules Health Checks
		urn = fmt.Sprintf("%s-fw-ingress-allow-health-checks", urnPrefix)
		_, err = compute.NewFirewall(ctx, urn, &compute.FirewallArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(urn),
			Description: pulumi.String("GKE at Scale - FW - Allow - Ingress - TCP Health Checks"),
			Network:     gcpNetwork.Name,
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports: pulumi.StringArray{
						pulumi.String("80"),
						pulumi.String("8080"),
						pulumi.String("443"),
					},
				},
			},
			SourceRanges: pulumi.StringArray{
				pulumi.String("35.191.0.0/16"),
				pulumi.String("130.211.0.0/22"),
			},
		})
		if err != nil {
			return err
		}

		// Create Firewall Rules Health Checks
		urn = fmt.Sprintf("%s-fw-ingress-allow-cluster-app-access", urnPrefix)
		_, err = compute.NewFirewall(ctx, urn, &compute.FirewallArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(urn),
			Description: pulumi.String("GKE at Scale - FW - Allow - Ingress - Load Balancer to Application"),
			Network:     gcpNetwork.Name,
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports: pulumi.StringArray{
						pulumi.String("80"),
						pulumi.String("8080"),
						pulumi.String("443"),
					},
				},
			},
			SourceRanges: pulumi.StringArray{
				pulumi.String("0.0.0.0/0"),
			},
			TargetTags: pulumi.StringArray{
				pulumi.String("gke-app-access"),
			},
		})
		if err != nil {
			return err
		}

		urn = fmt.Sprintf("%s-glb-tcp-health-check", urnPrefix)
		gcpGLBTCPHealthCheck, err := compute.NewHealthCheck(ctx, urn, &compute.HealthCheckArgs{
			Project:          pulumi.String(gcpProjectId),
			CheckIntervalSec: pulumi.Int(1),
			Description:      pulumi.String("TCP Health Check"),
			HealthyThreshold: pulumi.Int(4),
			TcpHealthCheck: &compute.HealthCheckTcpHealthCheckArgs{
				Port:        pulumi.Int(80),
				ProxyHeader: pulumi.String("NONE"),
			},
			TimeoutSec:         pulumi.Int(1),
			UnhealthyThreshold: pulumi.Int(5),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}

		var backendServiceBackendArray = compute.BackendServiceBackendArray{}
		urn = fmt.Sprintf("%s-glb-bes", urnPrefix)
		gcpBackendService, err := compute.NewBackendService(ctx, urn, &compute.BackendServiceArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-bes", urnPrefix)),
			Description: pulumi.String("GKE At Scale - Global Load Balancer - Backend Service"),
			CdnPolicy: &compute.BackendServiceCdnPolicyArgs{
				ClientTtl:  pulumi.Int(5),
				DefaultTtl: pulumi.Int(5),
				MaxTtl:     pulumi.Int(5),
			},
			ConnectionDrainingTimeoutSec: pulumi.Int(10),
			Backends:                     backendServiceBackendArray,
			HealthChecks:                 gcpGLBTCPHealthCheck.ID(),
		})
		if err != nil {
			return err
		}

		// Create URL Map
		urn = fmt.Sprintf("%s-glb-url-map-https", urnPrefix)
		gcpGLBURLMapHTTPS, err := compute.NewURLMap(ctx, urn, &compute.URLMapArgs{
			Project:        pulumi.String(gcpProjectId),
			Name:           pulumi.String(fmt.Sprintf("%s-glb-urlmap-https", urnPrefix)),
			Description:    pulumi.String("GKE At Scale - Global Load Balancer - HTTPS URL Map"),
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
			Description: pulumi.String("GKE At Scale - Global Load Balancer - HTTP URL Map"),
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

		// Process Each Cloud Region;
		for _, cloudRegion := range CloudRegions {
			if !cloudRegion.Enabled {
				// Logging Region Skipping
				fmt.Printf("[GAS INFO] - Cloud Region: %s - SKIPPING\n", cloudRegion.Region)
				continue
			}

			// Logging Region Processing
			fmt.Printf("[GAS INFO] - Cloud Region: %s - PROCESSING\n", cloudRegion.Region)

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
			cloudRegion.GKEClusterName = fmt.Sprintf("%s-gke-cluster-%s", urnPrefix, cloudRegion.Region)
			gcpGKECluster, err := container.NewCluster(ctx, cloudRegion.GKEClusterName, &container.ClusterArgs{
				Project:               pulumi.String(gcpProjectId),
				Name:                  pulumi.String(cloudRegion.GKEClusterName),
				Network:               gcpNetwork.ID(),
				Subnetwork:            gcpSubnetwork.ID(),
				Location:              pulumi.String(cloudRegion.Region),
				RemoveDefaultNodePool: pulumi.Bool(true),
				InitialNodeCount:      pulumi.Int(1),
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
				WorkloadIdentityConfig: &container.ClusterWorkloadIdentityConfigArgs{
					WorkloadPool: pulumi.String(fmt.Sprintf("%s.svc.id.goog", gcpProjectId)),
				},
			}, pulumi.IgnoreChanges([]string{"gatewayApiConfig"}))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-gke-%s-np-01", urnPrefix, cloudRegion.Region)
			gcpGKENodePool, err := container.NewNodePool(ctx, urn, &container.NodePoolArgs{
				Cluster:   gcpGKECluster.ID(),
				NodeCount: pulumi.Int(1),
				NodeConfig: &container.NodePoolNodeConfigArgs{
					Preemptible:    pulumi.Bool(false),
					MachineType:    pulumi.String("e2-medium"),
					ServiceAccount: gcpServiceAccount.Email,
					OauthScopes: pulumi.StringArray{
						pulumi.String("https://www.googleapis.com/auth/cloud-platform"),
					},
				},
				Autoscaling: &container.NodePoolAutoscalingArgs{
					LocationPolicy: pulumi.String("BALANCED"),
					MaxNodeCount:   pulumi.Int(5),
					MinNodeCount:   pulumi.Int(1),
				},
			})
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-kubeconfig", cloudRegion.GKEClusterName)
			k8sProvider, err := kubernetes.NewProvider(ctx, urn, &kubernetes.ProviderArgs{
				Kubeconfig: generateKubeconfig(gcpGKECluster.Endpoint, gcpGKECluster.Name, gcpGKECluster.MasterAuth),
			}, pulumi.DependsOn([]pulumi.Resource{gcpGKENodePool}))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-istio-base-%s", urnPrefix, cloudRegion.Region)
			helmIstioBase, err := helm.NewRelease(ctx, urn, &helm.ReleaseArgs{
				//ResourcePrefix: cloudRegion.Id,
				Description: pulumi.String("Istio Service Mesh - Install IstioBase"),
				RepositoryOpts: &helm.RepositoryOptsArgs{
					Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
				},
				Chart:           pulumi.String("base"),
				Namespace:       pulumi.String("istio-system"),
				CleanupOnFail:   pulumi.Bool(true),
				CreateNamespace: pulumi.Bool(true),
				Values: pulumi.Map{
					"defaultRevision": pulumi.String("default"),
				},
			}, pulumi.Provider(k8sProvider))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-istio-istiod-%s", urnPrefix, cloudRegion.Region)
			helmIstioD, err := helm.NewRelease(ctx, urn, &helm.ReleaseArgs{
				//ResourcePrefix: cloudRegion.Id,
				Description: pulumi.String("Istio Service Mesh - Install Istiod"),
				RepositoryOpts: &helm.RepositoryOptsArgs{
					Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
				},
				Chart:           pulumi.String("istiod"),
				Namespace:       pulumi.String("istio-system"),
				CleanupOnFail:   pulumi.Bool(true),
				CreateNamespace: pulumi.Bool(true),
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioBase}), pulumi.Parent(gcpGKENodePool))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-k8s-namespace-app-%s", urnPrefix, cloudRegion.Region)
			k8sAppNamespace, err := k8s.NewNamespace(ctx, urn, &k8s.NamespaceArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: pulumi.String("app-team"),
					Labels: pulumi.StringMap{
						"istio-injection": pulumi.String("enabled"),
					},
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioD}))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-istio-igw-%s", urnPrefix, cloudRegion.Region)
			_, err = helm.NewRelease(ctx, urn, &helm.ReleaseArgs{
				Name:        pulumi.String("istio-ingressgateway"),
				Description: pulumi.String("Istio Service Mesh - Install Ingress Gateway"),
				RepositoryOpts: &helm.RepositoryOptsArgs{
					Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
				},
				Chart:         pulumi.String("gateway"),
				Namespace:     k8sAppNamespace.Metadata.Name(),
				CleanupOnFail: pulumi.Bool(true),
				Values: pulumi.Map{
					"service": pulumi.Map{
						//"type": pulumi.String("LoadBalancer"),
						"type": pulumi.String("ClusterIP"),
						"annotations": pulumi.Map{
							"cloud.google.com/neg":                 pulumi.String("{\"exposed_ports\": {\"80\":{}}}"),
							"controller.autoneg.dev/neg":           pulumi.Sprintf("{\"backend_services\":{\"80\":[{\"name\":\"%s\",\"max_rate_per_endpoint\":100}]}}", gcpBackendService.Name),
							"networking.gke.io/load-balancer-type": pulumi.String("Internal"),
						},
					},
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioBase, helmIstioD}), pulumi.Parent(gcpGKENodePool))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-cluster-ops-%s", urnPrefix, cloudRegion.Region)
			helmClusterOps, err := helm.NewChart(ctx, urn, helm.ChartArgs{
				Chart:          pulumi.String("cluster-ops"),
				ResourcePrefix: cloudRegion.Id,
				Version:        pulumi.String("0.1.0"),
				Path:           pulumi.String("../../apps/helm"),
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
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioBase, helmIstioD}), pulumi.Parent(gcpGKENodePool))
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
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{gcpGKENodePool, helmClusterOps}), pulumi.Parent(gcpGKENodePool))
			if err != nil {
				return err
			}

			urn = fmt.Sprintf("%s-helm-deploy-app-team-%s", urnPrefix, cloudRegion.Region)
			_, err = helm.NewChart(ctx, urn, helm.ChartArgs{
				Chart:          pulumi.String("app-team"),
				ResourcePrefix: cloudRegion.Id,
				Version:        pulumi.String("0.1.1"),
				Path:           pulumi.String("../../apps/helm"),
				Values: pulumi.Map{
					"global": pulumi.Map{
						"labels": pulumi.Map{
							"region": pulumi.String(cloudRegion.Region),
						},
					},
					"app": pulumi.Map{
						"namespace":       k8sAppNamespace.Metadata.Name(),
						"customer":        pulumi.String("Pulumi Developer!"),
						"region":          pulumi.String(cloudRegion.Region),
						"colorPrimary":    pulumi.String("#805ac3"),
						"colorSecondary":  pulumi.String("#F6B436"),
						"colorBackground": pulumi.String("#FFFFFF"),
					},
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioBase, helmIstioD}), pulumi.Parent(gcpGKECluster))
			if err != nil {
				return err
			}
		}

		_ = gcpProject

		return nil
	})
}

func generateKubeconfig(clusterEndpoint pulumi.StringOutput, clusterName pulumi.StringOutput,
	clusterMasterAuth container.ClusterMasterAuthOutput) pulumi.StringOutput {
	context := pulumi.Sprintf("%s", clusterName)

	return pulumi.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://%s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
kind: Config
preferences: {}
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin
      installHint: Install gke-gcloud-auth-plugin for use with kubectl by following
        https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke
      provideClusterInfo: true
`,
		clusterMasterAuth.ClusterCaCertificate().Elem(),
		clusterEndpoint, context, context, context, context, context, context)
}
