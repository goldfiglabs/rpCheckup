provider "aws" {
  region = var.region
}

data "aws_caller_identity" "current" {}

resource "aws_iam_role" "rpcheckup" {
  name = var.role_name
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "AWS": "${data.aws_caller_identity.current.account_id}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_policy" "rpcheckup" {
  name = "rpCheckupExtraPermissions"
  policy = jsonencode(
    {
      "Statement" = [
        {
          Action = [
            "apigateway:GetRestApis",
            "efs:Describe*",
            "acm-pca:List*",
            "acm-pca:GetPolicy"
          ],
          Effect = "Allow",
          Resource = "*"
        }
      ],
      Version = "2012-10-17"
    }
  )
}

resource "aws_iam_role_policy_attachment" "extra_permissions" {
  role = aws_iam_role.rpcheckup.name
  policy_arn = aws_iam_policy.rpcheckup.arn
}

resource "aws_iam_role_policy_attachment" "security_audit" {
  role = aws_iam_role.rpcheckup.name
  policy_arn = "arn:aws:iam::aws:policy/SecurityAudit"
}

resource "aws_iam_role_policy_attachment" "view_only" {
  role = aws_iam_role.rpcheckup.name
  policy_arn = "arn:aws:iam::aws:policy/job-function/ViewOnlyAccess"
}
