
while getopts c:r:p:n: flag
do
    case "${flag}" in
        c) GKE_CLUSTER=${OPTARG};;
        r) GKE_REGION=${OPTARG};;
        p) GKE_PROJECT=${OPTARG};;
        n) GKE_PROJECT_NUMBER=${OPTARG};;
    esac
done
echo "GKE Cluster Name: $GKE_CLUSTER";
echo "GKE Cluster Region: $GKE_REGION";
echo "GKE Cluster Project: $GKE_PROJECT";
echo "GKE Cluster Project Number: $GKE_PROJECT_NUMBER";

rm -rf "./output/${GKE_CLUSTER}-${GKE_REGION}"

gcloud container clusters get-credentials ${GKE_CLUSTER} --region ${GKE_REGION} --project ${GKE_PROJECT}
gcloud container fleet memberships delete "${GKE_CLUSTER}-${GKE_REGION}" --location global --project ${GKE_PROJECT} --quiet
