package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {

	var (
		client          *kubernetes.Clientset
		deploymentLable map[string]string
		err             error
	)

	ctx := context.Background()

	if client, err = getClient(); err != nil {
		fmt.Printf("Error : &s", err)
		os.Exit(1)
	}
	if deploymentLable, err = deploy(ctx, client); err != nil {
		fmt.Printf("Error : &s", err)
		os.Exit(1)
	}
	if waitForPod(ctx, client, deploymentLable); err != nil {
		fmt.Printf("Error : &s", err)
		os.Exit(1)
	}
	fmt.Printf("deploy-finished,did a deployment with labels: %+v\n", deploymentLable)
}

func getClient() (*kubernetes.Clientset, error) {

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func deploy(ctx context.Context, client *kubernetes.Clientset) (map[string]string, error) {
	var deployment *v1.Deployment

	appFile, err := ioutil.ReadFile("app.yaml")
	if err != nil {
		return nil, fmt.Errorf("readfile error %s", err)
	}

	obj, groupVersionKind, err := scheme.Codecs.UniversalDeserializer().Decode(appFile, nil, nil)

	switch obj.(type) {
	case *v1.Deployment:
		deployment = obj.(*v1.Deployment)
	default:
		return nil, fmt.Errorf("Unrecognized type: %s\n", groupVersionKind)
	}

	_, err = client.AppsV1().Deployments("default").Get(ctx, deployment.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		deploymentResponse, err := client.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("deployment error %s", err)
		}
		return deploymentResponse.Spec.Template.Labels, nil

	} else if err != nil && errors.IsNotFound(err) {
		return nil, fmt.Errorf("deployment error: %s", err)
	}

	deploymentResponse, err := client.AppsV1().Deployments("default").Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("deployment error %s", err)
	}
	return deploymentResponse.Spec.Template.Labels, nil
}

func waitForPod(ctx context.Context, client *kubernetes.Clientset, deploymentLabels map[string]string) error {

	for {
		validateLables, err := labels.ValidatedSelectorFromSet(deploymentLabels)
		if err != nil {
			fmt.Errorf("Validate.SelectorFromSet error : %s", err)
		}
		podList, err := client.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: validateLables.String(),
		})
		if err != nil {
			return fmt.Errorf("pod list error : %s", err)
		}
		podsRunning := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == "Running" {
				podsRunning++
			}

		}
		if podsRunning > 0 && podsRunning == len(podList.Items) {
			break
		}
		fmt.Printf("waiting for pods to become ready (running %d/%d)\n", podsRunning, len(podList.Items))
		time.Sleep(5 * time.Second)

	}
	return nil
}
