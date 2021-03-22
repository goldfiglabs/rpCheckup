#!/bin/bash

set -e

if [[ -z $(which jq) ]]; then
  echo "script requires jq"
  exit 1
fi

if [[ -z $(which aws) ]]; then
  echo "script requires aws cli"
  exit 1
fi

if [[ -z $1 ]]; then
  echo "usage: ./run_with_role.sh <ROLE_ARN> [<SESSION_NAME>]"
  exit 2
fi

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  bin="rpCheckup_linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  arch=$(uname -m)
  if [[ $arch == "arm64" ]]; then
    bin="rpCheckup_darwin_arm64"
  else
    bin="rpCheckup_darwin_amd64"
  fi
else
  echo "Unsupported OS ${OSTYPE}"
  exit 3
fi

ROLE_ARN=$1
SESSION_NAME=${2:-rpCheckupSession}

cmd="aws sts assume-role --role-arn ${ROLE_ARN} --role-session-name ${SESSION_NAME}"
creds=$($cmd)
accessKeyId=$(echo $creds | jq -r '.Credentials.AccessKeyId')
secretKey=$(echo $creds | jq -r '.Credentials.SecretAccessKey')
sessionToken=$(echo $creds | jq -r '.Credentials.SessionToken')

AWS_ACCESS_KEY_ID=$accessKeyId \
  AWS_SECRET_ACCESS_KEY=$secretKey \
  AWS_SESSION_TOKEN=$sessionToken \
  dist/${bin}
