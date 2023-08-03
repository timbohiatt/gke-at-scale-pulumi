while getopts c:r:p:n:l: flag
do
    case "${flag}" in
        c) GKE_CLUSTER=${OPTARG};;
        r) GKE_REGION=${OPTARG};;
        p) GKE_PROJECT=${OPTARG};;
        n) GKE_PROJECT_NUMBER=${OPTARG};;
        l) GKE_IDX=${OPTARG};;
    esac
done

echo "GKE Cluster Name: $GKE_CLUSTER";
echo "GKE Cluster Region: $GKE_REGION";
echo "GKE Cluster Project: $GKE_PROJECT";
echo "GKE Cluster Project Number: $GKE_PROJECT_NUMBER";
echo "GKE Cluster Index?: $GKE_IDX";

rm -rf "./output/${GKE_CLUSTER}-${GKE_REGION}"

#echo "[ACTION - Get GKE Cluster Context and Credentials]"
gcloud container clusters get-credentials ${GKE_CLUSTER} --region ${GKE_REGION} --project ${GKE_PROJECT}

echo "[ACTION - Set GKE IAM Policy Bindings]"
gcloud projects add-iam-policy-binding ${GKE_PROJECT}  \
  --member "serviceAccount:service-${GKE_PROJECT_NUMBER}@gcp-sa-servicemesh.iam.gserviceaccount.com" \
  --role roles/anthosservicemesh.serviceAgent

# Update GKE Cluster Annotations
echo "[ACTION - Update GKE Cluster Annotations with Mesh ID]"
gcloud container clusters update ${GKE_CLUSTER} --region ${GKE_REGION} --project ${GKE_PROJECT} --update-labels mesh_id=proj-${GKE_PROJECT_NUMBER}

# Register GKE cluster in Fleet
echo "[ACTION - Register GKE Cluster in Fleet]"
gcloud container fleet memberships register "${GKE_CLUSTER}" --gke-cluster="${GKE_REGION}/${GKE_CLUSTER}" --enable-workload-identity --project ${GKE_PROJECT} --quiet

# Store Cluster Feet Memberhips ID
echo "[ACTION - Collect GKE Cluster Fleet Memberships]"

MEMBERSHIPS=$(gcloud container fleet memberships list --project ${GKE_PROJECT} --filter="${GKE_CLUSTER}" --format="value(NAME)")
echo $MEMBERSHIPS

# Fleet Config Cluster (when first cluster, index 0)
if [ "${GKE_IDX}" == "0" ]; then
    echo "[ACTION - Enabling Config Cluster with Fleet Ingress]"
    gcloud container fleet ingress enable --config-membership=${MEMBERSHIPS} --location=global --project=${GKE_PROJECT}
else
    echo "[SKIP - Fleet Ingress Registration]"
fi

# Fleet Mesh Update
echo "[ACTION - Update GKE Fleet Mesh with Memberships]"
gcloud container fleet mesh update --management automatic --memberships ${MEMBERSHIPS} --project ${GKE_PROJECT} --location global

# Prepare ASM Install
mkdir -p "./output/${GKE_CLUSTER}"

# Fleet Config Cluster (when first cluster, index 0)
if [ "${GKE_IDX}" == "0" ]; then
    echo "[ACTION - Configuring Firewall Rules from Lead Cluster]"

    # Create Firewall Rules for Cross Cluster Communication
    function join_by { local IFS="$1"; shift; echo "$*"; }
    ALL_CLUSTER_CIDRS=$(gcloud container clusters list --project ${GKE_PROJECT} --format='value(clusterIpv4Cidr)' | sort | uniq)
    ALL_CLUSTER_CIDRS=$(join_by , $(echo "${ALL_CLUSTER_CIDRS}"))
    ALL_CLUSTER_NETTAGS=$(gcloud compute instances list --project ${GKE_PROJECT} --format='value(tags.items.[0])' | sort | uniq)
    ALL_CLUSTER_NETTAGS=$(join_by , $(echo "${ALL_CLUSTER_NETTAGS}"))

    TAGS=""
    for CLUSTER in "${GKE_CLUSTER}"
    do
        NEWTAGS+=$(gcloud compute firewall-rules list --filter="Name:${GKE_CLUSTER}*" --project "${GKE_PROJECT}" --format="value(targetTags)" | uniq) && TAGS+=","
        TAGS+="${NEWTAGS//$'\n'/,}"
    done
    TAGS=${TAGS:1}
    echo "Network tags for pod ranges are $TAGS"

    # TODO Move to Pulumi
    echo "[ACTION - Creating Firewall Rules]"
    gcloud compute firewall-rules create asm-multicluster-pods \
        --allow=tcp,udp,icmp,esp,ah,sctp \
        --network="gas-vpc" \
        --direction=INGRESS \
        --priority=900 \
        --source-ranges="${ALL_CLUSTER_CIDRS}" \
        --target-tags="${TAGS}" \
        --project ${GKE_PROJECT}
else
    echo "[SKIP - Firewall Rules already Configured]"
fi


# Install ASM
echo "[ACTION - Install ASM on GKE Cluster: ${GKE_CLUSTER} Region: ${GKE_REGION} in Project: ${GKE_PROJECT}]"
./gke-config/asmcli install \
      -p ${GKE_PROJECT} \
      -l ${GKE_REGION} \
      -n ${GKE_CLUSTER} \
      --fleet_id ${GKE_PROJECT} \
      --managed \
      --verbose \
      --enable-all \
      --output_dir "./output/${GKE_CLUSTER}" \
      --channel rapid

# Patch MultiCluster Mode on the Cluster
kubectl patch configmap/asm-options -n istio-system --type merge -p '{"data":{"multicluster_mode":"connected"}}'

# Enable Global Access to Cluster
#gcloud container clusters update ${GKE_CLUSTER} --project ${GKE_PROJECT} --zone ${GKE_REGION} --enable-master-global-access
#gcloud beta container fleet config-management apply --membership ${MEMBERSHIPS} --project ${GKE_PROJECT} --config="./gke-config/acm.yaml"