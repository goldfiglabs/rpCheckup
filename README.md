# rpCheckup - Catch AWS resource policy backdoors like Endgame

![rpcheckup2](https://user-images.githubusercontent.com/291215/109551055-ee774e00-7a84-11eb-9242-606b7160eb1b.png)

rpCheckup is an AWS resource policy security checkup tool that identifies public, external account access, intra-org account access, and private resources. It makes it easy to reason about resource visibility across all the accounts in your org.

## Why?

We ([Gold Fig Labs](https://goldfiglabs.com)) built rpCheckup based on a part of how we assess customer AWS accounts. While there are many tools to assess and analyze IAM policies, the same treatment for policies attached to resources is a blind spot. As product iteration sometimes necessitates overprovisioned access to just get things working, finding such issues after the fact across a slew of different AWS resource types, accounts, and regions isn't straightforward.

rpCheckup generates an HTML & CSV report to make this easy.

## Supported AWS Resources

rpCheckup uses the resources supported by [Endgame](https://endgame.readthedocs.io/en/latest/) as the high-water mark for analyzing attached policies.

| Resource Type                                  | rpCheckup | Endgame | [AWS Access Analyzer][1] |
|------------------------------------------------|--------|---------|----------------------------------|
| ACM Private CAs                | ✅   | ✅     | ❌                               |
| CloudWatch Resource Policies      | ✅   | ✅     |  ❌                              |
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
| RDS DB Snapshots            | ✅   | ✅     | ❌                               |
| RDS Cluster Snapshots            | ✅   | ❌     |  ❌                              |
| S3 Buckets                          | ✅   | ✅     | ✅                               |
| Secrets Manager Secrets | ✅   | ✅     | ✅                               |
| SES Sender Authorization Policies  | ✅   | ✅     | ❌                               |
| SQS Queues                         | ✅   | ✅     | ✅                               |
| SNS Topics                         | ✅   | ✅     | ❌                               |

## Pre-requisites

* AWS credentials (~/.aws/, env variables, metadata server, etc)
* Docker
* If running from source; go version >= go1.15

## Installing

1. Download the latest [release](https://github.com/goldfiglabs/rpCheckup/releases):

  Linux:

    curl -Lo rpCheckup https://github.com/goldfiglabs/rpCheckup/releases/latest/download/rpCheckup_linux
    chmod a+x ./rpCheckup

  OSX x86:

    curl -Lo rpCheckup https://github.com/goldfiglabs/rpCheckup/releases/latest/download/rpCheckup_darwin_amd64
    chmod a+x ./rpCheckup

  OSX M1/arm:

    curl -Lo rpCheckup https://github.com/goldfiglabs/rpCheckup/releases/latest/download/rpCheckup_darwin_arm64
    chmod a+x ./rpCheckup

2. Run from source:
```
git clone https://github.com/goldfiglabs/rpCheckup.git
cd rpCheckup
go run main.go
```

## Usage

Run `./rpCheckup` and view the generated report found in `output/`.

<img width="800" alt="Screen Shot 2021-03-01 at 12 22 36 PM" src="https://user-images.githubusercontent.com/291215/109732631-61122780-7b72-11eb-8f6d-1b51758d2f19.png">

## Overview
rpCheckup uses [goldfiglabs/introspector](https://github.com/goldfiglabs/introspector) to snapshot the configuration of your AWS account. rpCheckup runs SQL queries to generate findings based on this snapshot. Introspector does the heavy lifting of importing and normalizing the configurations while rpCheckup is responsible for querying and report generation.

## Notes
If the account you are scanning is not the master account in an Organization, other
accounts in the Organization may be detected as external accounts. This is because
non-master accounts may not have access to see the organization structure.

Since rpCheckup relies on Introspector's snapshots, rpCheckup is unable to detect policies that are no longer attached. When detecting flapping or transient access, please use tools which utilize audit and security logs (CloudTrail, etc). See [here][2] for further information in preventing resource exposure.

## Sample Reports

See sample reports in `sample/`

<img width="1000" alt="Screen Shot 2021-02-26 at 9 59 12 PM" src="https://user-images.githubusercontent.com/291215/109552865-2bdcdb00-7a87-11eb-95a8-977269043f1d.png">

rpCheckup report against Endgame sample account:

<img width="1000" alt="Screen Shot 2021-03-02 at 4 05 40 PM" src="https://user-images.githubusercontent.com/291215/109732589-4c359400-7b72-11eb-979b-b673ed6d3449.png">


## License

Copyright (c) 2019-2021 [Gold Fig Labs Inc.](https://www.goldfiglabs.com/)

This Source Code Form is subject to the terms of the Mozilla Public License, v.2.0. If a copy of the MPL was not distributed with this file, You can obtain one at http://mozilla.org/MPL/2.0/.

[Mozilla Public License v2.0](./LICENSE)

[1]: https://docs.aws.amazon.com/IAM/latest/UserGuide/access-analyzer-resources.html
[2]: https://endgame.readthedocs.io/en/latest/prevention/
