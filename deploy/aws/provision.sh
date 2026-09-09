#!/usr/bin/env bash
# =============================================================================
# Provision the evolella.com backend on AWS EC2 (free plan credits).
#
# Prerequisites:
#   - AWS credentials available as env vars (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY)
#   - AWS CLI v2 installed
#
# Usage:
#   set -a; source ~/.growth-aws-creds; set +a
#   bash deploy/aws/provision.sh
#
# Creates: keypair, security group, t4g.large (Ubuntu 24.04 ARM), Elastic IP.
# Writes the public IP to deploy/aws/instance-ip.txt.
# =============================================================================
set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
KEY_NAME="growth-deploy"
SG_NAME="growth-prod-sg"
INSTANCE_NAME="evolella-prod"
# Free-plan accounts can only launch free-tier-eligible types.
# m7i-flex.large is the biggest allowed (2 vCPU / 4 GB, x86).
INSTANCE_TYPE="${INSTANCE_TYPE:-m7i-flex.large}"

AWS="${AWS_BIN:-aws}"

# --- Key pair ---
if ! "$AWS" ec2 describe-key-pairs --region "$REGION" --key-names "$KEY_NAME" >/dev/null 2>&1; then
  "$AWS" ec2 create-key-pair --key-name "$KEY_NAME" --region "$REGION" \
    --query 'KeyMaterial' --output text > ~/.ssh/"$KEY_NAME".pem
  chmod 600 ~/.ssh/"$KEY_NAME".pem
  echo "Created keypair $KEY_NAME -> ~/.ssh/$KEY_NAME.pem"
else
  echo "Keypair $KEY_NAME already exists"
fi

# --- Default VPC ---
vpc_id=$("$AWS" ec2 describe-vpcs --region "$REGION" \
  --filters Name=is-default,Values=true \
  --query 'Vpcs[0].VpcId' --output text)
echo "Default VPC: $vpc_id"

# --- Security group ---
SG_ID=$("$AWS" ec2 describe-security-groups --region "$REGION" \
  --filters Name=group-name,Values="$SG_NAME" \
  --query 'SecurityGroups[0].GroupId' --output text 2>/dev/null || true)
if [ -z "$SG_ID" ] || [ "$SG_ID" = "None" ]; then
  SG_ID=$("$AWS" ec2 create-security-group --region "$REGION" \
    --group-name "$SG_NAME" \
    --description "evolella.com prod: Caddy 80/443, SSH" \
    --vpc-id "$vpc_id" --query 'GroupId' --output text)
  "$AWS" ec2 authorize-security-group-ingress --group-id "$SG_ID" --region "$REGION" \
    --protocol tcp --port 22 --cidr 0.0.0.0/0 >/dev/null
  "$AWS" ec2 authorize-security-group-ingress --group-id "$SG_ID" --region "$REGION" \
    --protocol tcp --port 80 --cidr 0.0.0.0/0 >/dev/null
  "$AWS" ec2 authorize-security-group-ingress --group-id "$SG_ID" --region "$REGION" \
    --protocol tcp --port 443 --cidr 0.0.0.0/0 >/dev/null
  echo "Created security group $SG_ID (22/80/443)"
else
  echo "Security group exists: $SG_ID"
fi

# --- Latest Ubuntu 24.04 LTS AMD64 AMI (Canonical-owned; m7i-flex is x86) ---
ami_id=$("$AWS" ec2 describe-images --region "$REGION" \
  --owners 099720109477 \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*" \
            "Name=state,Values=available" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' --output text)
echo "Using AMI: $ami_id"

# --- Launch instance ---
instance_id=$("$AWS" ec2 run-instances --region "$REGION" \
  --image-id "$ami_id" \
  --instance-type "$INSTANCE_TYPE" \
  --key-name "$KEY_NAME" \
  --security-group-ids "$SG_ID" \
  --block-device-mappings "DeviceName=/dev/sda1,Ebs={VolumeSize=40,VolumeType=gp3,DeleteOnTermination=true}" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=$INSTANCE_NAME}]" \
  --metadata-options "HttpTokens=required,HttpPutResponseHopLimit=1" \
  --query 'Instances[0].InstanceId' --output text)
echo "Launched instance $instance_id"

echo "Waiting for instance to be running..."
"$AWS" ec2 wait instance-running --region "$REGION" --instance-ids "$instance_id"

# --- Elastic IP (stable address for DNS) ---
alloc_id=$("$AWS" ec2 allocate-address --region "$REGION" --domain vpc \
  --query 'AllocationId' --output text)
"$AWS" ec2 associate-address --region "$REGION" \
  --instance-id "$instance_id" --allocation-id "$alloc_id" >/dev/null
public_ip=$("$AWS" ec2 describe-addresses --region "$REGION" \
  --allocation-ids "$alloc_id" \
  --query 'Addresses[0].PublicIp' --output text)

echo ""
echo "=================================================="
echo " Instance:  $INSTANCE_NAME ($instance_id)"
echo " Public IP: $public_ip"
echo " SSH:       ssh -i ~/.ssh/$KEY_NAME.pem ubuntu@$public_ip"
echo "=================================================="
echo "$public_ip" > "$(dirname "$0")/instance-ip.txt"
