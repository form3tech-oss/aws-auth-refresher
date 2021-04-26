// Copyright 2019 Form3 Financial Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

type AWSAuthUser struct {
	Groups   []string `yaml:"groups,omitempty"`
	UserARN  string   `yaml:"userarn,omitempty"`
	Username string   `yaml:"username,omitempty"`
}

type AwsAuthUserSelector struct {
	ARNRegex string   `yaml:"arnRegex,omitempty"`
	Groups   []string `yaml:"groups,omitempty"`
	Username string   `yaml:"username,omitempty"`
}

// buildAwsAuthMapUsersEntry build the final value of the 'mapUsers' entry of the 'kube-system/aws-auth' ConfigMap.
func buildAwsAuthMapUsersEntry(us []AwsAuthUserSelector, iu []*iam.User) ([]AWSAuthUser, error) {
	res := make([]AWSAuthUser, 0)
	for _, v := range us {
		r, err := regexp.Compile(v.ARNRegex)
		if err != nil {
			return nil, fmt.Errorf("failed to compile ARN regex: %w", err)
		}
		for _, u := range iu {
			if u.Arn != nil && r.MatchString(*u.Arn) {
				res = append(res, AWSAuthUser{
					Groups:   v.Groups,
					UserARN:  *u.Arn,
					Username: v.Username,
				})
			}
		}
	}

	return res, nil
}

// createKubeClient creates a Kubernetes client based on the specified kubeconfig file.
func createKubeClient(pathToKubeconfig string) (kubernetes.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", pathToKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	c, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return c, nil
}

// listAWSIAMUsers returns a list of all AWS IAM users that currently exist.
func listAWSIAMUsers(iamClient iamiface.IAMAPI) ([]*iam.User, error) {
	var (
		marker *string
		users  = make([]*iam.User, 0)
	)
	for {
		o, err := iamClient.ListUsers(&iam.ListUsersInput{
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}
		users = append(users, o.Users...)
		if o.IsTruncated != nil && !*o.IsTruncated {
			break
		} else {
			marker = o.Marker
		}
	}

	return users, nil
}

// refreshAwsAuthConfigMap refreshes the 'kube-system/aws-auth' ConfigMap based on the 'kube-system/aws-auth-refresher' ConfigMap and the current list of AWS IAM users.
func refreshAwsAuthConfigMap(kubeClient kubernetes.Interface, iamClient iamiface.IAMAPI) {
	// Read the 'kube-system/aws-auth-refresher' ConfigMap.
	s, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get("aws-auth-refresher", metav1.GetOptions{})
	if err != nil {
		log.Error(err)

		return
	}
	// Read the value of the 'mapUsers' entry.
	d, exists := s.Data["mapUsers"]
	if !exists {
		log.Debugf("No rules found")

		return
	}
	// Decode the value of the 'mapUsers' entry.
	var us []AwsAuthUserSelector
	if err := yaml.NewDecoder(strings.NewReader(d)).Decode(&us); err != nil {
		log.Error(err)

		return
	}
	// Grab an up-to-date list of AWS IAM users.
	u, err := listAWSIAMUsers(iamClient)
	if err != nil {
		log.Error(err)

		return
	}
	// Build the final value of the 'mapUsers' entry of the 'kube-system/aws-auth-refresher' ConfigMap.
	l, err := buildAwsAuthMapUsersEntry(us, u)
	if err != nil {
		log.Error(err)

		return
	}
	// Read the 'kube-system/aws-auth' ConfigMap.
	t, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get("aws-auth", metav1.GetOptions{})
	if err != nil {
		log.Error(err)

		return
	}
	// Encode the value of the 'mapUsers' entry.
	var b strings.Builder
	if err := yaml.NewEncoder(&b).Encode(l); err != nil {
		log.Error(err)

		return
	}
	// Update the 'mapUsers' entry of the 'kube-system/aws-auth' ConfigMap.
	t.Data["mapUsers"] = b.String()
	if _, err := kubeClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Update(t); err != nil {
		log.Error(err)

		return
	}
}

func main() {
	// Parse command-line flags.
	logLevel := flag.String("log-level", log.InfoLevel.String(), "the log level to use")
	pathToKubeconfig := flag.String("path-to-kubeconfig", "", "the path to the kubeconfig file to use")
	refreshInterval := flag.Duration("refresh-interval", time.Duration(15)*time.Second, "the interval at which to refresh the 'aws-auth' configmap")
	flag.Parse()

	// Configure logging.
	if v, err := log.ParseLevel(*logLevel); err != nil {
		log.Fatalf("Failed to parse log level: %v", err)
	} else {
		log.SetLevel(v)
	}
	klog.SetOutput(ioutil.Discard)

	// Create a Kubernetes configuration object and client.
	k, err := createKubeClient(*pathToKubeconfig)
	if err != nil {
		log.Fatalf("Failed to build Kubernetes client: %v", err)
	}

	// Initialize the AWS IAM client.
	s, err := session.NewSession()
	if err != nil {
		log.Fatalf("Failed to initialize AWS session: %v", err)
	}
	c := iam.New(s)

	// Setup a signal handler for SIGINT and SIGTERM so we can gracefully shutdown when requested to.
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Refresh the 'kube-system/aws-auth' ConfigMap whenever the specified refresh interval elapses.
	t := time.NewTicker(*refreshInterval)
	defer t.Stop()
	refreshAwsAuthConfigMap(k, c)
	for {
		select {
		case <-stopCh:
			return
		case <-t.C:
			refreshAwsAuthConfigMap(k, c)
		}
	}
}
