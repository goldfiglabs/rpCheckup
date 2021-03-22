# rpCheckup Terraform permissions role

This Terraform module creates a role in the AWS account you are currently signed in to. Review the permissions in `main.tf`.

Use Terraform to create the role in your account:

1. terraform init
2. terraform plan
3. terraform apply

Upon success, the Terraform run will output an ARN. This can be used by `run_with_role.sh` to invoke rpCheckup with the newly created role.
