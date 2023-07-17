
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

gcloud container clusters get-credentials ${GKE_CLUSTER} --region ${GKE_REGION} --project ${GKE_PROJECT}
gcloud container fleet memberships delete "${GKE_CLUSTER}-${GKE_REGION}" --location ${GKE_REGION} --project ${GKE_PROJECT} --quiet
sleep 45s


# DELETE ALL NETWORK ENDPOINT GROUPS