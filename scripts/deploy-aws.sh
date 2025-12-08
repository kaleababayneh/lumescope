#!/usr/bin/env bash
#
# deploy-aws.sh – Deploy LumeScope to AWS App Runner
#
# This script:
#   1. Builds the Docker image locally
#   2. Pushes it to Amazon ECR
#   3. Creates/updates an App Runner service
#
# Prerequisites:
#   - AWS CLI configured with appropriate credentials
#   - Docker installed and running
#
# Usage:
#   ./scripts/deploy-aws.sh [OPTIONS]
#
# Options:
#   --region REGION         AWS region (default: us-east-1)
#   --service-name NAME     App Runner service name (default: lumescope)
#   --lumera-api URL        Lumera API base URL (default: https://api.lumera.com:1317)
#   --dry-run               Show what would be done without executing
#   --help                  Show this help message

set -euo pipefail

# --------------------------------------------------
# Default configuration
# --------------------------------------------------
AWS_REGION="${AWS_REGION:-us-east-1}"
SERVICE_NAME="lumescope"
LUMERA_API_BASE="https://api.lumera.com:1317"
IMAGE_NAME="lumescope"
IMAGE_TAG="latest"
APP_PORT=18080
DRY_RUN=false

# Background worker intervals (defaults match .env)
VALIDATORS_SYNC_INTERVAL="30m"
SUPERNODES_SYNC_INTERVAL="20m"
ACTIONS_SYNC_INTERVAL="5m"
PROBE_INTERVAL="10m"

# --------------------------------------------------
# Color output helpers
# --------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }
die()     { error "$@"; exit 1; }

# --------------------------------------------------
# Usage
# --------------------------------------------------
usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Deploy LumeScope to AWS App Runner with ephemeral PostgreSQL database.

Options:
  --region REGION                   AWS region (default: us-east-1)
  --service-name NAME               App Runner service name (default: lumescope)
  --lumera-api URL                  Lumera API base URL (default: https://api.lumera.com:1317)
  --validators-sync-interval INTERVAL   Validators sync interval (default: 30m)
  --supernodes-sync-interval INTERVAL   Supernodes sync interval (default: 20m)
  --actions-sync-interval INTERVAL      Actions sync interval (default: 5m)
  --probe-interval INTERVAL             Probe interval (default: 10m)
  --dry-run                         Show what would be done without executing
  --help                            Show this help message

Environment Variables:
  AWS_REGION              Override default region
  AWS_PROFILE             AWS CLI profile to use

Examples:
  # Deploy with defaults
  ./scripts/deploy-aws.sh

  # Deploy to a specific region with custom API endpoint
  ./scripts/deploy-aws.sh --region eu-west-1 --lumera-api https://custom-api.example.com:1317

  # Deploy with custom sync intervals
  ./scripts/deploy-aws.sh --validators-sync-interval 1h --probe-interval 5m

  # Dry run to see what would happen
  ./scripts/deploy-aws.sh --dry-run
EOF
    exit 0
}

# --------------------------------------------------
# Parse arguments
# --------------------------------------------------
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --region)
                AWS_REGION="$2"
                shift 2
                ;;
            --service-name)
                SERVICE_NAME="$2"
                shift 2
                ;;
            --lumera-api)
                LUMERA_API_BASE="$2"
                shift 2
                ;;
            --validators-sync-interval)
                VALIDATORS_SYNC_INTERVAL="$2"
                shift 2
                ;;
            --supernodes-sync-interval)
                SUPERNODES_SYNC_INTERVAL="$2"
                shift 2
                ;;
            --actions-sync-interval)
                ACTIONS_SYNC_INTERVAL="$2"
                shift 2
                ;;
            --probe-interval)
                PROBE_INTERVAL="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help|-h)
                usage
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
}

# --------------------------------------------------
# Run or dry-run a command
# --------------------------------------------------
run_cmd() {
    if [[ "$DRY_RUN" == "true" ]]; then
        echo -e "${YELLOW}[DRY-RUN]${NC} $*"
    else
        "$@"
    fi
}

# --------------------------------------------------
# Check prerequisites
# --------------------------------------------------
check_prerequisites() {
    info "Checking prerequisites..."
    
    if ! command -v aws &>/dev/null; then
        die "AWS CLI is not installed. Please install it: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
    fi
    
    if ! command -v docker &>/dev/null; then
        die "Docker is not installed. Please install it: https://docs.docker.com/get-docker/"
    fi
    
    # Check if Docker daemon is running
    if ! docker info &>/dev/null; then
        die "Docker daemon is not running. Please start Docker."
    fi
    
    # Verify AWS credentials
    if ! aws sts get-caller-identity &>/dev/null; then
        die "AWS credentials not configured or invalid. Run 'aws configure' or set AWS_PROFILE."
    fi
    
    success "Prerequisites check passed"
}

# --------------------------------------------------
# Get AWS account info
# --------------------------------------------------
setup_aws_vars() {
    info "Setting up AWS variables..."
    
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    ECR_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
    ECR_IMAGE_URI="${ECR_REPO}/${IMAGE_NAME}:${IMAGE_TAG}"
    
    info "AWS Account: ${AWS_ACCOUNT_ID}"
    info "AWS Region: ${AWS_REGION}"
    info "ECR Repository: ${ECR_REPO}"
    info "Image URI: ${ECR_IMAGE_URI}"
}

# --------------------------------------------------
# Build Docker image
# --------------------------------------------------
build_image() {
    info "Building Docker image: ${IMAGE_NAME}:${IMAGE_TAG}"
    
    # Build from project root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
    
    run_cmd docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" "$PROJECT_ROOT"
    
    success "Docker image built successfully"
}

# --------------------------------------------------
# Push image to ECR
# --------------------------------------------------
push_to_ecr() {
    info "Pushing image to ECR..."
    
    # Login to ECR
    info "Logging in to ECR..."
    if [[ "$DRY_RUN" != "true" ]]; then
        aws ecr get-login-password --region "${AWS_REGION}" | \
            docker login --username AWS --password-stdin "${ECR_REPO}"
    else
        echo -e "${YELLOW}[DRY-RUN]${NC} aws ecr get-login-password | docker login"
    fi
    
    # Create ECR repository if it doesn't exist
    info "Ensuring ECR repository exists..."
    if [[ "$DRY_RUN" != "true" ]]; then
        if ! aws ecr describe-repositories --repository-names "${IMAGE_NAME}" --region "${AWS_REGION}" &>/dev/null; then
            info "Creating ECR repository: ${IMAGE_NAME}"
            aws ecr create-repository \
                --repository-name "${IMAGE_NAME}" \
                --region "${AWS_REGION}" \
                --image-scanning-configuration scanOnPush=true
            success "ECR repository created"
        else
            info "ECR repository already exists"
        fi
    else
        echo -e "${YELLOW}[DRY-RUN]${NC} aws ecr create-repository --repository-name ${IMAGE_NAME}"
    fi
    
    # Tag and push
    info "Tagging image for ECR..."
    run_cmd docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "${ECR_IMAGE_URI}"
    
    info "Pushing image to ECR..."
    run_cmd docker push "${ECR_IMAGE_URI}"
    
    success "Image pushed to ECR: ${ECR_IMAGE_URI}"
}

# --------------------------------------------------
# Create/Update IAM role for App Runner
# --------------------------------------------------
setup_iam_role() {
    info "Setting up IAM role for App Runner..."
    
    ROLE_NAME="AppRunnerECRAccessRole-${SERVICE_NAME}"
    
    # Trust policy for App Runner
    TRUST_POLICY=$(cat <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "build.apprunner.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
EOF
)
    
    if [[ "$DRY_RUN" != "true" ]]; then
        # Check if role exists
        if aws iam get-role --role-name "${ROLE_NAME}" &>/dev/null; then
            info "IAM role already exists: ${ROLE_NAME}"
        else
            info "Creating IAM role: ${ROLE_NAME}"
            aws iam create-role \
                --role-name "${ROLE_NAME}" \
                --assume-role-policy-document "${TRUST_POLICY}"
            
            # Attach ECR read policy
            aws iam attach-role-policy \
                --role-name "${ROLE_NAME}" \
                --policy-arn "arn:aws:iam::aws:policy/service-role/AWSAppRunnerServicePolicyForECRAccess"
            
            # Wait for role to propagate
            info "Waiting for IAM role to propagate..."
            sleep 10
            
            success "IAM role created and policy attached"
        fi
        
        ROLE_ARN=$(aws iam get-role --role-name "${ROLE_NAME}" --query 'Role.Arn' --output text)
    else
        echo -e "${YELLOW}[DRY-RUN]${NC} aws iam create-role --role-name ${ROLE_NAME}"
        echo -e "${YELLOW}[DRY-RUN]${NC} aws iam attach-role-policy --role-name ${ROLE_NAME} --policy-arn arn:aws:iam::aws:policy/service-role/AWSAppRunnerServicePolicyForECRAccess"
        ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"
    fi
    
    info "IAM Role ARN: ${ROLE_ARN}"
}

# --------------------------------------------------
# Deploy to App Runner
# --------------------------------------------------
deploy_app_runner() {
    info "Deploying to App Runner..."
    
    # Check if service exists
    if [[ "$DRY_RUN" != "true" ]]; then
        SERVICE_EXISTS=$(aws apprunner list-services --region "${AWS_REGION}" \
            --query "ServiceSummaryList[?ServiceName=='${SERVICE_NAME}'].ServiceArn" \
            --output text 2>/dev/null || echo "")
    else
        SERVICE_EXISTS=""
    fi
    
    # Build source configuration
    SOURCE_CONFIG=$(cat <<EOF
{
    "ImageRepository": {
        "ImageIdentifier": "${ECR_IMAGE_URI}",
        "ImageConfiguration": {
            "Port": "${APP_PORT}",
            "RuntimeEnvironmentVariables": {
                "LUMERA_API_BASE": "${LUMERA_API_BASE}",
                "PORT": "${APP_PORT}",
                "VALIDATORS_SYNC_INTERVAL": "${VALIDATORS_SYNC_INTERVAL}",
                "SUPERNODES_SYNC_INTERVAL": "${SUPERNODES_SYNC_INTERVAL}",
                "ACTIONS_SYNC_INTERVAL": "${ACTIONS_SYNC_INTERVAL}",
                "PROBE_INTERVAL": "${PROBE_INTERVAL}"
            }
        },
        "ImageRepositoryType": "ECR"
    },
    "AuthenticationConfiguration": {
        "AccessRoleArn": "${ROLE_ARN}"
    }
}
EOF
)
    
    # Health check configuration
    HEALTH_CHECK_CONFIG=$(cat <<EOF
{
    "Protocol": "HTTP",
    "Path": "/healthz",
    "Interval": 10,
    "Timeout": 5,
    "HealthyThreshold": 1,
    "UnhealthyThreshold": 5
}
EOF
)
    
    # Instance configuration
    INSTANCE_CONFIG=$(cat <<EOF
{
    "Cpu": "1024",
    "Memory": "2048"
}
EOF
)
    
    if [[ -n "$SERVICE_EXISTS" ]]; then
        info "Updating existing App Runner service: ${SERVICE_NAME}"
        
        if [[ "$DRY_RUN" != "true" ]]; then
            aws apprunner update-service \
                --region "${AWS_REGION}" \
                --service-arn "${SERVICE_EXISTS}" \
                --source-configuration "${SOURCE_CONFIG}" \
                --health-check-configuration "${HEALTH_CHECK_CONFIG}" \
                --instance-configuration "${INSTANCE_CONFIG}"
            
            success "App Runner service update initiated"
            
            # Get service URL
            SERVICE_URL=$(aws apprunner describe-service \
                --region "${AWS_REGION}" \
                --service-arn "${SERVICE_EXISTS}" \
                --query 'Service.ServiceUrl' \
                --output text)
        else
            echo -e "${YELLOW}[DRY-RUN]${NC} aws apprunner update-service --service-arn ${SERVICE_EXISTS}"
            SERVICE_URL="<service-url>"
        fi
    else
        info "Creating new App Runner service: ${SERVICE_NAME}"
        
        if [[ "$DRY_RUN" != "true" ]]; then
            CREATE_OUTPUT=$(aws apprunner create-service \
                --region "${AWS_REGION}" \
                --service-name "${SERVICE_NAME}" \
                --source-configuration "${SOURCE_CONFIG}" \
                --health-check-configuration "${HEALTH_CHECK_CONFIG}" \
                --instance-configuration "${INSTANCE_CONFIG}")
            
            SERVICE_ARN=$(echo "${CREATE_OUTPUT}" | jq -r '.Service.ServiceArn')
            SERVICE_URL=$(echo "${CREATE_OUTPUT}" | jq -r '.Service.ServiceUrl')
            
            success "App Runner service created"
        else
            echo -e "${YELLOW}[DRY-RUN]${NC} aws apprunner create-service --service-name ${SERVICE_NAME}"
            SERVICE_URL="<service-url>"
        fi
    fi
    
    info "Service URL: https://${SERVICE_URL}"
}

# --------------------------------------------------
# Print summary
# --------------------------------------------------
print_summary() {
    echo ""
    echo "=============================================="
    echo "           Deployment Summary"
    echo "=============================================="
    echo ""
    echo "Service Name:     ${SERVICE_NAME}"
    echo "AWS Region:       ${AWS_REGION}"
    echo "ECR Image:        ${ECR_IMAGE_URI}"
    echo "Lumera API:       ${LUMERA_API_BASE}"
    echo "App Port:         ${APP_PORT}"
    echo ""
    echo "Background Worker Intervals:"
    echo "  Validators Sync: ${VALIDATORS_SYNC_INTERVAL}"
    echo "  Supernodes Sync: ${SUPERNODES_SYNC_INTERVAL}"
    echo "  Actions Sync:    ${ACTIONS_SYNC_INTERVAL}"
    echo "  Probe:           ${PROBE_INTERVAL}"
    echo ""
    if [[ "$DRY_RUN" != "true" ]]; then
        echo "Service URL:      https://${SERVICE_URL}"
        echo ""
        echo "Endpoints:"
        echo "  Health:         https://${SERVICE_URL}/healthz"
        echo "  Ready:          https://${SERVICE_URL}/readyz"
        echo "  API Docs:       https://${SERVICE_URL}/docs"
        echo "  Supernodes:     https://${SERVICE_URL}/api/v1/supernodes"
        echo ""
        echo "Note: It may take a few minutes for the service to be fully available."
    else
        echo -e "${YELLOW}[DRY-RUN] No changes were made.${NC}"
    fi
    echo "=============================================="
}

# --------------------------------------------------
# Main
# --------------------------------------------------
main() {
    parse_args "$@"
    
    echo ""
    info "Starting LumeScope deployment to AWS App Runner"
    echo ""
    
    if [[ "$DRY_RUN" == "true" ]]; then
        warn "Running in DRY-RUN mode - no changes will be made"
        echo ""
    fi
    
    check_prerequisites
    setup_aws_vars
    build_image
    push_to_ecr
    setup_iam_role
    deploy_app_runner
    print_summary
    
    success "Deployment complete!"
}

main "$@"