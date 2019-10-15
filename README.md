# aws-auth-refresher

[![Build Status](https://travis-ci.com/form3tech-oss/aws-auth-refresher.svg?branch=master)](https://travis-ci.com/form3tech-oss/aws-auth-refresher)

Makes it easier to use AWS EKS with temporary AWS IAM users.

## Motivation

[Authentication in AWS EKS](https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html) works only with a fixed, well-known set of AWS IAM users (and roles).
This makes it hard to use temporary, dynamically-created users/security credentials (such as when using [Vault](https://www.vaultproject.io/)), with AWS EKS.
`aws-auth-refresher` makes it possible to define `kube-system/aws-auth` in terms of regular expressions (see the [example](#example)).
It periodically matches these regular expressions against existing AWS IAM users, and updates the `kube-system/aws-auth` ConfigMap accordingly.

## Prerequisites

* The `iam:ListUsers` permission attached to your AWS EKS worker nodes.

## Installing

### Helm (Experimental)

To install `aws-auth-refresher` using Helm, run

```shell
$ helm repo add aws-auth-refresher https://form3tech-oss.github.io/aws-auth-refresher
```

```shell
$ helm repo update
```

```shell
$ helm upgrade --install aws-auth-refresher aws-auth-refresher/aws-auth-refresher
```

Please check [`values.yaml`](https://github.com/form3tech-oss/aws-auth-refresher/blob/master/helm/aws-auth-refresher/values.yaml) for details on how to tweak the installation. 

### `kubectl`

To install `aws-auth-refresher` using `kubectl, run

```shell
$ kubectl apply -f deploy/common.yaml
```

```shell
$ kubectl apply -f deploy/deployment.yaml
```

## Example

To configure `aws-auth-refresher`, create an `aws-auth-refresher` ConfigMap in the `kube-system` namespace with a content similar to the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth-refresher
  namespace: kube-system
data:
  mapUsers: |
    - arnRegex: ^arn:aws:iam::1234567890:user/tmp-foo-.*$
      username: foo
      groups:
      - system:masters
    - arnRegex: ^arn:aws:iam::1234567890:user/tmp-bar-.*$
      username: bar
      groups:
      - developers
```

Given this configuration, and assuming only the `tmp-foo-one` AWS IAM user initially exists, `aws-auth-refresher` will update `kube-system/aws-auth` with the following content:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth-refresher
  namespace: kube-system
data:
  mapUsers: |
    - userarn: arn:aws:iam::1234567890:user/tmp-foo-one
      username: foo
      groups:
      - system:masters
```

Then, assuming `tmp-bar-one` and `tmp-bar-two` get created some time after this, `aws-auth-refresher` will update `kube-system/aws-auth` with the following content:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth-refresher
  namespace: kube-system
data:
  mapUsers: |
    - userarn: arn:aws:iam::1234567890:user/tmp-foo-one
      username: foo
      groups:
      - system:masters
    - userarn: arn:aws:iam::1234567890:user/tmp-bar-one
      username: bar
      groups:
      - developers
    - userarn: arn:aws:iam::1234567890:user/tmp-bar-two
      username: bar
      groups:
      - developers
```

When the `tmp-foo-one` and `tmp-bar-two` AWS IAM users get deleted, `aws-auth-refresher` will update `kube-system/aws-auth` with the following content:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth-refresher
  namespace: kube-system
data:
  mapUsers: |
    - userarn: arn:aws:iam::1234567890:user/tmp-bar-one
      username: bar
      groups:
      - developers
 ```

## License
Copyright 2019 Form3 Financial Cloud

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.