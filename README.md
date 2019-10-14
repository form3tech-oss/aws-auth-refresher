# aws-auth-refresher

Makes it easier to use AWS EKS with temporary AWS IAM users.

## Motivation

[Authentication in AWS EKS](https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html) works only with a fixed, well-known set of AWS IAM users (and roles).
This makes it hard to use temporary, dynamically-created users/security credentials (such as when using [Vault](https://www.vaultproject.io/)), with AWS EKS.
`aws-auth-refresher` makes it possible to define `kube-system/aws-auth` in terms of regular expressions (see the [example](#example)).
It periodically matches these regular expressions against existing AWS IAM users, and updates the `kube-system/aws-auth` ConfigMap accordingly.

## Prerequisites

* The `iam:ListUsers` permission attached to your AWS EKS worker nodes.

## Installing

To install `aws-auth-refresher`, start by running

```shell
$ kubectl apply -f deploy/common.yaml
serviceaccount/aws-auth-refresher created
role.rbac.authorization.k8s.io/aws-auth-refresher created
rolebinding.rbac.authorization.k8s.io/aws-auth-refresher created
```

Then, run

```shell
kubectl apply -f deploy/deployment.yaml
deployment.apps/aws-auth-refresher created
```

and make sure that `aws-auth-refresher` is indeed running:


```shell
kubectl -n kube-system get pod -l app=aws-auth-refresher
NAME                                  READY   STATUS    RESTARTS   AGE
aws-auth-refresher-566cb9bf88-vdj46   1/1     Running   0          2s
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