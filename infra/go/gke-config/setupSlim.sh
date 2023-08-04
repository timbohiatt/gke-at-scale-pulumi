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

# Enable Global Access to Cluster
#gcloud container clusters update ${GKE_CLUSTER} --project ${GKE_PROJECT} --zone ${GKE_REGION} --enable-master-global-access
#gcloud beta container fleet config-management apply --membership ${MEMBERSHIPS} --project ${GKE_PROJECT} --config="./gke-config/acm.yaml"