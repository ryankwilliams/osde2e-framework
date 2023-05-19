variable "aws_region" {
  type        = string
  description = "The region to create the ROSA cluster in"
  default     = "us-east-2"

  validation {
    condition     = contains(["us-east-2", "us-west-2", "eu-central-1"], var.aws_region)
    error_message = "HyperShift is currently only available in these regions: us-east-2, eu-west-1."
  }
}

variable "az_ids" {
  type = object({
    us-east-2 = list(string)
    us-west-2 = list(string)
    eu-central-1 = list(string)
  })
  description = "A list of region-mapped AZ IDs that a subnet should get deployed into"
  default = {
    us-east-2 = ["use2-az1", "use2-az2"]
    us-west-2 = ["usw2-az1", "usw2-az2"]
    eu-central-1 = ["euc1-az1", "euc1-az2"]
  }
}

variable "cluster_name" {
  type        = string
  description = "The name of the ROSA cluster to create"
  default     = "rosa-cluster"

  validation {
    condition     = can(regex("^[a-z][-a-z0-9]{0,13}[a-z0-9]$", var.cluster_name))
    error_message = "ROSA cluster name must be less than 16 characters, be lower case alphanumeric, with only hyphens."
  }
}

provider "aws" {
  region = var.aws_region
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = ">= 3.14.2, < 4.0.0"

  name = "${var.cluster_name}-vpc"
  cidr = "10.0.0.0/16"

  azs             = var.az_ids[var.aws_region]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true
}

output "cluster-private-subnet" {
  value = module.vpc.private_subnets[0]
}

output "cluster-public-subnet" {
  value = module.vpc.public_subnets[0]
}

output "node-private-subnet" {
  value = module.vpc.private_subnets[1]
}