# rpCheckup - Resource Policy Checkup for AWS

rpCheckup is an AWS resource policy security checkup tool that identifies public, external account access, intra-org account access, and private resources. It makes it easy to reason about resource visibility across all the accounts in your org.

## Why?

While there are many tools to assess and analyze IAM policies, the same treatment for policies attached to resources is a blind spot. As product iteration sometimes necessitates overprivisioned access to just get things working, finding such issues after the fact across a slew of different AWS resource types, accounts, and regions isn't straightfoward. 

rpCheckup generates an HTML or CSV report to make this easy.

## Supported AWS Resources

rpCheckup uses the resources supported by [Endgame](https://endgame.readthedocs.io/en/latest/) as the high-water mark for analyzing attached policies.

| Resource Type                                  | rpCheckup | Endgame | [AWS Access Analyzer][1] |
|------------------------------------------------|--------|---------|----------------------------------|
| ACM Private CAs                | ✅   | ✅     | ❌                               |
| CloudWatch Resource Policies      | ✅   | ✅     | ❌                               |
| EBS Volume Snapshots               | ✅   | ✅     | ❌                               |
| EC2 AMIs                          | ✅   | ✅     | ❌                               |
| ECR Container Repositories         | ✅   | ✅     | ❌                               |
| EFS File Systems                   | ✅   | ✅     | ❌                               |
| ElasticSearch Domains               | ✅   | ✅     | ❌                               |
| Glacier Vault Access Policies  | ✅   | ✅     | ❌                               |
| IAM Roles                    | ✅   | ✅     | ✅                               |
| KMS Keys                           | ✅   | ✅     | ✅                               |
| Lambda Functions                                        | ✅   | ✅     | ✅                               |
| Lambda Layers            | ✅   | ✅     | ✅                               |
| RDS Snapshots            | ✅   | ✅     | ❌                               |
| S3 Buckets                          | ✅   | ✅     | ✅                               |
| Secrets Manager Secrets | ✅   | ✅     | ✅                               |
| SES Sender Authorization Policies  | ✅   | ✅     | ❌                               |
| SQS Queues                         | ✅   | ✅     | ✅                               |
| SNS Topics                         | ✅   | ✅     | ❌                               |

## Pre-requisities

* boto findable credentials (~/.aws/ etc)
* Docker version > 19.03.13, build 4484c46d9d
* If running from source; go version go1.16 darwin/amd64


## Installing

Pre built binaries:
    
    curl -O release/rpCheckup


Run from source:

    git clone https://github.com/goldfiglabs/rpCheckup.git
    cd rpCheckup
    go run main.go

## Usage

Run `./rpCheckup` and view the generated report. 

TODO: flags for generating CSVs.

## Overview
rpCheckup uses goldfiglabs/introspector#1 to snapshot the configuration of your AWS account. rpCheckup runs SQL queries to generate findings based on this snapshot. Introspector does the heavy lifting of importing and normalizing the configurations while rpCheckup is responsible for querying and report generation. 

TODO: Describe down-scoped credential found in goldfiglabs/introspector?

## Notes
Since rpCheckup relies on Introspector's snapshots, rpCheckup is unable to detect policies that are no longer attached. When detecting flapping or transient access, please use tools which utilize audit and security logs (CloudTrail, etc). See [here][2] for further information in preventing resource exposure. 

TODO: Add example runs against Endgame Terraform'd account.

## License

Copyright (c) 2019-2021 Gold Fig Labs Inc.

This Source Code Form is subject to the terms of the Mozilla Public License, v.2.0. If a copy of the MPL was not distributed with this file, You can obtain one at http://mozilla.org/MPL/2.0/.

[Mozilla Public License v2.0](./LICENSE)



[1]: https://docs.aws.amazon.com/IAM/latest/UserGuide/access-analyzer-resources.html
[2]: https://endgame.readthedocs.io/en/latest/prevention/#inventory-which-iam-principals-are-capable-of-resource-exposure
