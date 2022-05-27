/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
)

type options struct {
	k8sclient *kubernetes.Clientset
	k8sdynamic dynamic.Interface
}
var opt options

// contentsCmd represents the contents command
var contentsCmd = &cobra.Command{
	Use:   "contents",
	Short: "get contents of the bundle",
	Long: `get contents of the bundle

rukpakctl bundle contents <bundle name> display contents of the bundle.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		contents(opt, args)
	},
}

func init() {
	bundleCmd.AddCommand(contentsCmd)
	homeDir := os.Getenv("HOME")
	config, err := clientcmd.BuildConfigFromFlags("", homeDir + "/.kube/config")
	if err != nil {
		fmt.Printf("failed to create kubernetes client config: %+v\n", err)
	}
	if opt.k8sclient, err = kubernetes.NewForConfig(config); err != nil {
		fmt.Printf("failed to create kubernetes client: %+v\n", err)
	}
	if opt.k8sdynamic, err = dynamic.NewForConfig(config); err != nil {
		fmt.Printf("failed to create dynamic kubernetes client: %+v\n", err)
	}
}

func contents(opt options, args []string) error {
	// Create a temporary ServiceAccount
	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:  "rukpakctl-sa",
		},
	}
	deployedSa, err := opt.k8sclient.CoreV1().ServiceAccounts("default").Create(context.Background(), &sa, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create a service account: %+v", err)
	}
	defer opt.k8sclient.CoreV1().ServiceAccounts("default").Delete(context.Background(), "rukpakctl-sa", metav1.DeleteOptions{})

	// Create a temporary ClusterRoleBinding to bind the ServiceAccount to bundle-reader ClusterRole
	crb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:  "rukpakctl-crb",
			Namespace:  "default",
		},
		Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "rukpakctl-sa", Namespace: "default"}},
		RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "bundle-reader", },
	}
	_, err = opt.k8sclient.RbacV1().ClusterRoleBindings().Create(context.Background(), &crb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create a cluster role bindings: %+v", err)
	}
	defer opt.k8sclient.RbacV1().ClusterRoleBindings().Delete(context.Background(), "rukpakctl-crb", metav1.DeleteOptions{})

	// Wait for token Secret is created and attached to the ServiceAccount
	for {
		deployedSa, err = opt.k8sclient.CoreV1().ServiceAccounts("default").Get(context.Background(), "rukpakctl-sa", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get a service account: %+v", err)
			break
		}
		if len(deployedSa.Secrets) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if len(deployedSa.Secrets) == 0 {
		return fmt.Errorf("failed to get access token")
	}

	// Get contentURL in the Bundle
	bundleGVR := schema.GroupVersionResource{Group: "core.rukpak.io", Version: "v1alpha1", Resource: "bundles",}
	bundleObj, err := opt.k8sdynamic.Resource(bundleGVR).Get(context.Background(), args[0], metav1.GetOptions{})
	if err != nil || bundleObj == nil{
		return fmt.Errorf("failed to get the bundle: %+v", err)
	}

	bundleUnstructured := bundleObj.UnstructuredContent()
	var bundle rukpakv1alpha1.Bundle
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(bundleUnstructured, &bundle)
	if err != nil {
		return fmt.Errorf("error : %+v", err)
	}
	url := bundle.Status.ContentURL

	// Create a Job that reada from the URL and outputs contents in the pod log
	mounttoken := true
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:  "rukpakctl-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "rukpakctl",
							Image: "curlimages/curl",
							Command: []string{"sh", "-c", "curl -sSLk -H \"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" -o - " + url + " | tar ztv"},
						},
					},
					ServiceAccountName: "rukpakctl-sa",
					RestartPolicy: "Never",
					AutomountServiceAccountToken: &mounttoken,
					
				},
			},
		},
	}
	_, err = opt.k8sclient.BatchV1().Jobs("default").Create(context.Background(), &job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create a job: %+v", err)
	}
	option := metav1.DeletePropagationBackground
	defer  opt.k8sclient.BatchV1().Jobs("default").Delete(context.Background(), "rukpakctl-job", metav1.DeleteOptions{PropagationPolicy: &option,})

	// Wait for Job completion
	for {
		deployedJob, err := opt.k8sclient.BatchV1().Jobs("default").Get(context.Background(), "rukpakctl-job", metav1.GetOptions{})
		if err != nil {
			fmt.Errorf("failed to get a service account: %+v", err)
			break
		}
		if deployedJob.Status.CompletionTime != nil  {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Get Pod for the Job
	pods, err := opt.k8sclient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{LabelSelector: "job-name=rukpakctl-job",})
	if err != nil {
		return fmt.Errorf("failed to find pods for job: %+v", err)
	}
	if len(pods.Items) != 1 {
		return fmt.Errorf("There are more than 1 pod found for the job\n")
	}
	defer opt.k8sclient.CoreV1().Pods("default").Delete(context.Background(), pods.Items[0].Name, metav1.DeleteOptions{})

	// Get logs of the Pod
	logReader, err := opt.k8sclient.CoreV1().Pods("default").GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).Stream(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get pod logs: %+v", err)
	}
	defer logReader.Close()
	if _, err := io.Copy(os.Stdout, logReader); err != nil {
		return fmt.Errorf("Failed to read log: %+v", err)
	}

	return nil
}