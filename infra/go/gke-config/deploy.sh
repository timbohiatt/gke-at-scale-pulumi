
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

gcloud container clusters get-credentials ${GKE_CLUSTER} --region ${GKE_REGION} --project ${GKE_PROJECT}

kubectl apply -f ./gke-config/k8s
