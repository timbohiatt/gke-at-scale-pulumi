package main

import (
	"errors"
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/container"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/iam"
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
		Enabled:  true,
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
		Enabled:  true,
		Region:   "asia-east1",
		SubnetIp: "10.128.150.0/24",
	},
	cloudRegion{
		Id:       "004",
		Enabled:  false,
		Region:   "australia-southeast1",
		SubnetIp: "10.128.200.0/24",
	},
	cloudRegion{
		Id:       "005",
		Enabled:  false,
		Region:   "me-west1",
		SubnetIp: "10.128.250.0/24",
	},
	cloudRegion{
		Id:       "006",
		Enabled:  false,
		Region:   "southamerica-west1",
		SubnetIp: "10.129.50.0/24",
	},
	cloudRegion{
		Id:       "007",
		Enabled:  false,
		Region:   "europe-north1",
		SubnetIp: "10.129.100.0/24",
	},
	cloudRegion{
		Id:       "008",
		Enabled:  false,
		Region:   "northamerica-northeast1",
		SubnetIp: "10.129.150.0/24",
	},
	cloudRegion{
		Id:       "009",
		Enabled:  false,
		Region:   "us-east4",
		SubnetIp: "10.129.200.0/24",
	},
	cloudRegion{
		Id:       "010",
		Enabled:  false,
		Region:   "us-east5",
		SubnetIp: "10.129.250.0/24",
	},
	cloudRegion{
		Id:       "011",
		Enabled:  false,
		Region:   "us-south1",
		SubnetIp: "10.130.50.0/24",
	},
	cloudRegion{
		Id:       "012",
		Enabled:  false,
		Region:   "europe-west8",
		SubnetIp: "10.130.100.0/24",
	},
	cloudRegion{
		Id:       "013",
		Enabled:  false,
		Region:   "europe-west9",
		SubnetIp: "10.130.150.0/24",
	},
	cloudRegion{
		Id:       "014",
		Enabled:  false,
		Region:   "europe-west3",
		SubnetIp: "10.130.200.0/24",
	},
	cloudRegion{
		Id:       "015",
		Enabled:  false,
		Region:   "europe-central2",
		SubnetIp: "10.130.250.0/24",
	},
}

// Declare an Array of API's To Enable.
var GCPServices = []string{
	"compute.googleapis.com",
	"container.googleapis.com",
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		// Global Variables
		var SSL bool
		gcpDependencies := []pulumi.Resource{}

		// Instanciate Pulumi Configuration
		cfg := config.New(ctx, "")

		// Review Google Cloud Project ID
		gcpProjectId := config.Get(ctx, "gcp:project")
		if gcpProjectId == "" {
			return errors.New("[CONFIGURATION] - [gcp:project] - No GCP Project Set: Pulumi GCP Provider must have Project configured")
		}

		// Review Prefix Configuration
		resourceNamePrefix := cfg.Get("prefix")
		if resourceNamePrefix == "" {
			return errors.New("[CONFIGURATION] - No Prefix has been provided; Please set a prefix (3-5 characters long), it is mandatory")
		} else {
			if len(resourceNamePrefix) > 5 {
				return fmt.Errorf("[CONFIGURATION] - Prefix: '%s' must be less than 5 characters in length", resourceNamePrefix)
			}
			fmt.Printf("[CONFIGURATION] - Prefix: %s has been provided; All Google Cloud resource names will be prefixed.\n", resourceNamePrefix)
		}

		// Review Domain & SSL Configuration
		domain := cfg.Get("domainName")
		if domain != "" {
			fmt.Printf("[CONFIGURATION] - Domain: '%s' has been provided; SSL Certificates will be configured for this domain.\n", domain)
			fmt.Printf("[CONFIGURATION] - DNS: The DNS for the domain: '%s' must be configured to point to the IP Address of the Global Load Balancer.\n", domain)
			SSL = true
		} else {
			fmt.Printf("[CONFIGURATION] - No Domain has been provided; Therefore HTTPS will not be enabled for this deployment.\n")
			SSL = false
		}

		// Enable Google API's on the Specified Project.
		for _, Service := range GCPServices {
			resourceName := fmt.Sprintf("%s-project-service-%s", resourceNamePrefix, Service)
			gcpService, err := projects.NewService(ctx, resourceName, &projects.ServiceArgs{
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
		resourceName := fmt.Sprintf("%s-glb-ip-address", resourceNamePrefix)
		gcpGlobalAddress, err := compute.NewGlobalAddress(ctx, resourceName, &compute.GlobalAddressArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(resourceName),
			AddressType: pulumi.String("EXTERNAL"),
			IpVersion:   pulumi.String("IPV4"),
			Description: pulumi.String("GKE At Scale - Global Load Balancer - Static IP Address"),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}
		// Export the Global Load Balancer IP Address
		ctx.Export(resourceName, gcpGlobalAddress.Address)

		// Create Custom IAM Role that will be used by the AutoNeg Kubernetes Deployment
		// This Role allows the AutoNeg CRD to link the Istio Ingress Gateway Service Ip to Load Balancer NEGs
		resourceName = fmt.Sprintf("%s-iam-custom-role-autoneg", resourceNamePrefix)
		gcpIAMRoleAutoNeg, err := projects.NewIAMCustomRole(ctx, resourceName, &projects.IAMCustomRoleArgs{
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

			RoleId: pulumi.String(fmt.Sprintf("%s_iam_role_autoneg_system", resourceNamePrefix)),
			Title:  pulumi.String("GKE at Scale - AutoNEG"),
		})
		if err != nil {
			return err
		}

		// Create Google Cloud Service Account
		resourceName = fmt.Sprintf("%s-service-account", resourceNamePrefix)
		gcpServiceAccount, err := serviceaccount.NewAccount(ctx, resourceName, &serviceaccount.AccountArgs{
			Project:     pulumi.String(gcpProjectId),
			AccountId:   pulumi.String("svc-gke-at-scale-admin"),
			DisplayName: pulumi.String("GKE at Scale - Admin Service Account"),
		})
		if err != nil {
			return err
		}

		// Create AutoNeg Service Account
		resourceName = fmt.Sprintf("%s-service-account-autoneg", resourceNamePrefix)
		gcpServiceAccountAutoNeg, err := serviceaccount.NewAccount(ctx, resourceName, &serviceaccount.AccountArgs{
			Project:     pulumi.String(gcpProjectId),
			AccountId:   pulumi.String("autoneg-system"),
			DisplayName: pulumi.String("GKE at Scale - AutoNEG Service Account"),
		})
		if err != nil {
			return err
		}

		// Create AutoNEG IAM Role Binding to link AutoNeg Service Account to Custom Role.
		resourceName = fmt.Sprintf("%s-iam-role-binding-autoneg", resourceNamePrefix)
		_, err = projects.NewIAMBinding(ctx, resourceName, &projects.IAMBindingArgs{
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
		resourceName = fmt.Sprintf("%s-wip-gke-cluster", resourceNamePrefix)
		_, err = iam.NewWorkloadIdentityPool(ctx, resourceName, &iam.WorkloadIdentityPoolArgs{
			Project:                pulumi.String(gcpProjectId),
			Description:            pulumi.String("GKE at Scale - Workload Identity Pool for GKE Cluster"),
			Disabled:               pulumi.Bool(false),
			DisplayName:            pulumi.String(resourceName),
			WorkloadIdentityPoolId: pulumi.String(fmt.Sprintf("%s-wip-gke-0019", resourceNamePrefix)), // **** TODO: Replace with Pulumi RANDOM ID? ****
		})
		if err != nil {
			return err
		}

		// Create Google Cloud VPC Network (Global Resource)
		resourceName = fmt.Sprintf("%s-vpc", resourceNamePrefix)
		gcpNetwork, err := compute.NewNetwork(ctx, resourceName, &compute.NetworkArgs{
			Project:               pulumi.String(gcpProjectId),
			Name:                  pulumi.String(resourceName),
			Description:           pulumi.String("GKE at Scale - Global VPC Network"),
			AutoCreateSubnetworks: pulumi.Bool(false),
		}, pulumi.DependsOn(gcpDependencies))
		if err != nil {
			return err
		}

		// Create Firewall Rules Health Checks (Network Endpoints within Load Balancer)
		resourceName = fmt.Sprintf("%s-fw-in-allow-health-checks", resourceNamePrefix)
		_, err = compute.NewFirewall(ctx, resourceName, &compute.FirewallArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(resourceName),
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

		// Create Firewall Rules - Inbound Cluster Access
		resourceName = fmt.Sprintf("%s-fw-in-allow-cluster-app", resourceNamePrefix)
		_, err = compute.NewFirewall(ctx, resourceName, &compute.FirewallArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(resourceName),
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

		// Create Health Checks (Network Endpoints within Load Balancer)
		resourceName = fmt.Sprintf("%s-glb-tcp-hc", resourceNamePrefix)
		gcpGLBTCPHealthCheck, err := compute.NewHealthCheck(ctx, resourceName, &compute.HealthCheckArgs{
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

		// Create Global Load Balancer Backend Service
		var backendServiceBackendArray = compute.BackendServiceBackendArray{}
		resourceName = fmt.Sprintf("%s-glb-bes", resourceNamePrefix)
		gcpBackendService, err := compute.NewBackendService(ctx, resourceName, &compute.BackendServiceArgs{
			Project:     pulumi.String(gcpProjectId),
			Name:        pulumi.String(fmt.Sprintf("%s-bes", resourceNamePrefix)),
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

		// Create Managed SSL Certificate
		if SSL {
			resourceName = fmt.Sprintf("%s-glb-ssl-cert", resourceNamePrefix)
			gcpGLBManagedSSLCert, err := compute.NewManagedSslCertificate(ctx, resourceName, &compute.ManagedSslCertificateArgs{
				Project:     pulumi.String(gcpProjectId),
				Name:        pulumi.String(resourceName),
				Description: pulumi.String("GKE at Scale - Global Load Balancer - Managed SSL Certificate"),
				Type:        pulumi.String("MANAGED"),
				Managed: &compute.ManagedSslCertificateManagedArgs{
					Domains: pulumi.StringArray{
						pulumi.String(domain),
					},
				},
			}, pulumi.DependsOn(gcpDependencies))
			if err != nil {
				return err
			}

			// Create URL Map
			resourceName = fmt.Sprintf("%s-glb-url-map-https-domain", resourceNamePrefix)
			gcpGLBURLMapHTTPS, err := compute.NewURLMap(ctx, resourceName, &compute.URLMapArgs{
				Project:        pulumi.String(gcpProjectId),
				Name:           pulumi.String(fmt.Sprintf("%s-glb-urlmap-https", resourceNamePrefix)),
				Description:    pulumi.String("GKE At Scale - Global Load Balancer - HTTPS URL Map"),
				DefaultService: gcpBackendService.SelfLink,
			})
			if err != nil {
				return err
			}

			// Create Target HTTPS Proxy
			resourceName = fmt.Sprintf("%s-glb-https-proxy", resourceNamePrefix)
			gcpGLBTargetHTTPSProxy, err := compute.NewTargetHttpsProxy(ctx, resourceName, &compute.TargetHttpsProxyArgs{
				Project: pulumi.String(gcpProjectId),
				Name:    pulumi.String(resourceName),
				UrlMap:  gcpGLBURLMapHTTPS.SelfLink,
				SslCertificates: pulumi.StringArray{
					gcpGLBManagedSSLCert.SelfLink,
				},
			})
			if err != nil {
				return err
			}

			// Global Load Balancer Forwarding Rule for HTTPS Traffic.
			resourceName = fmt.Sprintf("%s-glb-https-fwd-rule", resourceNamePrefix)
			_, err = compute.NewGlobalForwardingRule(ctx, resourceName, &compute.GlobalForwardingRuleArgs{
				Project:             pulumi.String(gcpProjectId),
				Target:              gcpGLBTargetHTTPSProxy.SelfLink,
				IpAddress:           gcpGlobalAddress.SelfLink,
				PortRange:           pulumi.String("443"),
				LoadBalancingScheme: pulumi.String("EXTERNAL"),
			})
			if err != nil {
				return err
			}

		}

		// Create URL Maps
		gcpGLBURLMapHTTP := &compute.URLMap{}
		if domain == "" {
			// Create URL Map - When No Domain is provided - HTTP Traffic.
			resourceName = fmt.Sprintf("%s-glb-url-map-http-no-domain", resourceNamePrefix)
			gcpGLBURLMapHTTP, err = compute.NewURLMap(ctx, resourceName, &compute.URLMapArgs{
				Project:        pulumi.String(gcpProjectId),
				Name:           pulumi.String(fmt.Sprintf("%s-glb-urlmap-http", resourceNamePrefix)),
				Description:    pulumi.String("GKE At Scale - Global Load Balancer - HTTP URL Map"),
				DefaultService: gcpBackendService.SelfLink,
			})
			if err != nil {
				return err
			}

		} else {
			// Create URL Map - When Domain is provided - HTTP Traffic.
			resourceName = fmt.Sprintf("%s-glb-url-map-http-domain", resourceNamePrefix)
			gcpGLBURLMapHTTP, err = compute.NewURLMap(ctx, resourceName, &compute.URLMapArgs{
				Project:     pulumi.String(gcpProjectId),
				Name:        pulumi.String(fmt.Sprintf("%s-glb-urlmap-http", resourceNamePrefix)),
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
									StripQuery: pulumi.Bool(false),
									// If Domain Configured and SSL Enabled
									HttpsRedirect: pulumi.Bool(SSL),
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
		}

		// Create Target HTTP Proxy
		resourceName = fmt.Sprintf("%s-glb-http-proxy", resourceNamePrefix)
		gcpGLBTargetHTTPProxy, err := compute.NewTargetHttpProxy(ctx, resourceName, &compute.TargetHttpProxyArgs{
			Project: pulumi.String(gcpProjectId),
			Name:    pulumi.String(resourceName),
			UrlMap:  gcpGLBURLMapHTTP.SelfLink,
		})
		if err != nil {
			return err
		}

		// Create HTTP Global Forwarding Rule
		resourceName = fmt.Sprintf("%s-glb-http-fwd-rule", resourceNamePrefix)
		_, err = compute.NewGlobalForwardingRule(ctx, resourceName, &compute.GlobalForwardingRuleArgs{
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
				fmt.Printf("[ INFORMATION ] - Cloud Region: %s - SKIPPING\n", cloudRegion.Region)
				continue
			}

			// Logging Region Processing
			fmt.Printf("[ INFORMATION ] - Cloud Region: %s - PROCESSING\n", cloudRegion.Region)

			// Create VPC Subnet for Cloud Region
			resourceName := fmt.Sprintf("%s-vpc-subnet-%s", resourceNamePrefix, cloudRegion.Region)
			gcpSubnetwork, err := compute.NewSubnetwork(ctx, resourceName, &compute.SubnetworkArgs{
				Project:               pulumi.String(gcpProjectId),
				Name:                  pulumi.String(resourceName),
				Description:           pulumi.String(fmt.Sprintf("GKE at Scale - VPC Subnet - %s", cloudRegion.Region)),
				IpCidrRange:           pulumi.String(cloudRegion.SubnetIp),
				Region:                pulumi.String(cloudRegion.Region),
				Network:               gcpNetwork.ID(),
				PrivateIpGoogleAccess: pulumi.Bool(true),
			})
			if err != nil {
				return err
			}

			// Create GKE Cluster for Cloud Region
			cloudRegion.GKEClusterName = fmt.Sprintf("%s-gke-%s", resourceNamePrefix, cloudRegion.Region)
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

			// Create GKE Node Pool
			resourceName = fmt.Sprintf("%s-gke-%s-np-01", resourceNamePrefix, cloudRegion.Region)
			gcpGKENodePool, err := container.NewNodePool(ctx, resourceName, &container.NodePoolArgs{
				Cluster:   gcpGKECluster.ID(),
				Name:      pulumi.String(resourceName),
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

			// Create New Kubernetes Provider for Each Cloud Region
			resourceName = fmt.Sprintf("%s-kubeconfig", cloudRegion.GKEClusterName)
			k8sProvider, err := kubernetes.NewProvider(ctx, resourceName, &kubernetes.ProviderArgs{
				Kubeconfig: generateKubeconfig(gcpGKECluster.Endpoint, gcpGKECluster.Name, gcpGKECluster.MasterAuth),
			}, pulumi.DependsOn([]pulumi.Resource{gcpGKENodePool}))
			if err != nil {
				return err
			}

			// Install Istio Service Mesh Base
			resourceName = fmt.Sprintf("%s-istio-base-%s", resourceNamePrefix, cloudRegion.Region)
			helmIstioBase, err := helm.NewRelease(ctx, resourceName, &helm.ReleaseArgs{
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

			// Install Istio Service Mesh Istiod
			resourceName = fmt.Sprintf("%s-istio-istiod-%s", resourceNamePrefix, cloudRegion.Region)
			helmIstioD, err := helm.NewRelease(ctx, resourceName, &helm.ReleaseArgs{
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

			// Create New Namespace in the GKE Clusters for Application Deployments
			resourceName = fmt.Sprintf("%s-k8s-ns-app-%s", resourceNamePrefix, cloudRegion.Region)
			k8sAppNamespace, err := k8s.NewNamespace(ctx, resourceName, &k8s.NamespaceArgs{
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

			// Deploy Istio Ingress Gateway into the GKE Clusters
			resourceName = fmt.Sprintf("%s-istio-igw-%s", resourceNamePrefix, cloudRegion.Region)
			_, err = helm.NewRelease(ctx, resourceName, &helm.ReleaseArgs{
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

			// Deploy Cluster Ops components for GKE AutoNeg
			resourceName = fmt.Sprintf("%s-cluster-ops-%s", resourceNamePrefix, cloudRegion.Region)
			helmClusterOps, err := helm.NewChart(ctx, resourceName, helm.ChartArgs{
				Chart:          pulumi.String("cluster-ops"),
				ResourcePrefix: cloudRegion.Id,
				Version:        pulumi.String("0.1.0"),
				Path:           pulumi.String("../apps/helm"),
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

			// Bind Kubernetes AutoNeg Service Account to Workload Identity
			resourceName = fmt.Sprintf("%s-iam-svc-k8s-%s", resourceNamePrefix, cloudRegion.Region)
			_, err = serviceaccount.NewIAMBinding(ctx, resourceName, &serviceaccount.IAMBindingArgs{
				ServiceAccountId: gcpServiceAccountAutoNeg.Name,
				Role:             pulumi.String("roles/iam.workloadIdentityUser"),
				Members: pulumi.StringArray{
					pulumi.String(fmt.Sprintf("serviceAccount:%s.svc.id.goog[autoneg-system/autoneg-controller-manager]", gcpProjectId)),
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{gcpGKENodePool, helmClusterOps}), pulumi.Parent(gcpGKENodePool))
			if err != nil {
				return err
			}

			// Deploy Application Team Applications
			resourceName = fmt.Sprintf("%s-app-%s", resourceNamePrefix, cloudRegion.Region)
			_, err = helm.NewChart(ctx, resourceName, helm.ChartArgs{
				Chart:          pulumi.String("app-team"),
				ResourcePrefix: cloudRegion.Id,
				Version:        pulumi.String("0.1.0"),
				Path:           pulumi.String("../apps/helm"),
				Values: pulumi.Map{
					"global": pulumi.Map{
						"labels": pulumi.Map{
							"region":  pulumi.String(cloudRegion.Region),
							"project": pulumi.String(gcpProjectId),
							"prefix":  pulumi.String(resourceNamePrefix),
						},
					},
					"deployment": pulumi.Map{
						"env": pulumi.Map{
							"customer":         pulumi.String("Pulumi Developers"),
							"color_primary":    pulumi.String("#805ac3"),
							"color_secondary":  pulumi.String("#4d5bd9"),
							"color_background": pulumi.String("#f7bf2a"),
							"location":         pulumi.String(cloudRegion.Region),
							"platform":         pulumi.String("GKE"),
						},
					},
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{helmIstioBase, helmIstioD}), pulumi.Parent(gcpGKECluster))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// Function - Generate KubeConfig that will be used by Pulumi Kubernetes
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
