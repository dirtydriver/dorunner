#!/bin/bash
# Cloud / deploy CLIs used by the workflows:
#   - AWS CLI v2 (aws)     — most-used tool across deploy/destroy/integration jobs
#   - AWS SAM CLI (sam)    — serverless build/deploy for Lambda artifacts
#   - Azure CLI (az)       — azure/login@v3 uses it but does not install it
#   - GitHub CLI (gh)      — release download, gh release create, gh pr merge
set -euxo pipefail

cd /tmp

# --- AWS CLI v2 -------------------------------------------------------------
curl -fsSL 'https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip' -o awscliv2.zip
unzip -oq awscliv2.zip
./aws/install
rm -rf awscliv2.zip aws/
aws --version

# --- AWS SAM CLI ------------------------------------------------------------
# Distributed as a zip with a bundled installer, mirroring the AWS CLI v2 flow.
curl -fsSL 'https://github.com/aws/aws-sam-cli/releases/latest/download/aws-sam-cli-linux-x86_64.zip' -o aws-sam-cli.zip
unzip -oq aws-sam-cli.zip -d sam-installation
./sam-installation/install
rm -rf aws-sam-cli.zip sam-installation/
sam --version

# --- Azure CLI --------------------------------------------------------------
# azure/login@v3 uses az but does not install it.
curl -fsSL https://aka.ms/InstallAzureCLIDeb | bash
az version

# --- GitHub CLI -------------------------------------------------------------
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo 'deb [arch=amd64 signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main' \
  > /etc/apt/sources.list.d/github-cli.list
apt-get update -y
apt-get install -y gh
gh --version
