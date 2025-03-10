package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/ohsu-comp-bio/funnel/tes"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// KubernetesCommand is responsible for configuring and running a task in a Kubernetes cluster.
type KubernetesCommand struct {
	TaskId       string
	JobId        int
	StdinFile    string
	TaskTemplate string
	Namespace    string
	Resources    *tes.Resources
	Command
}

// Creates a new Kuberntes Job which will run the task.
func (kcmd KubernetesCommand) Run(ctx context.Context) error {
	var taskId = kcmd.TaskId
	tpl, err := template.New(taskId).Parse(kcmd.TaskTemplate)

	if err != nil {
		return err
	}

	var command = kcmd.ShellCommand
	if kcmd.StdinFile != "" {
		command = append(command, "<", kcmd.StdinFile)
	}

	joinedCommand := strings.Join(command, " ")
	escapedCommand := strings.ReplaceAll(joinedCommand, `"`, `\"`)

	var buf bytes.Buffer
	err = tpl.Execute(&buf, map[string]interface{}{
		"TaskId":    taskId,
		"JobId":     kcmd.JobId,
		"Namespace": kcmd.Namespace,
		"Image":     kcmd.Image,
		"Command":   escapedCommand,
		"Env":       kcmd.Env,
		"Workdir":   kcmd.Workdir,
		"Volumes":   kcmd.Volumes,
		"Cpus":      kcmd.Resources.CpuCores,
		"RamGb":     kcmd.Resources.RamGb,
		"DiskGb":    kcmd.Resources.DiskGb,
	})

	if err != nil {
		return err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(buf.Bytes(), nil, nil)
	if err != nil {
		return err
	}

	job, ok := obj.(*v1.Job)
	if !ok {
		return err
	}

	clientset, err := getKubernetesClientset()
	if err != nil {
		return err
	}

	var client = clientset.BatchV1().Jobs(kcmd.Namespace)

	_, err = client.Create(ctx, job, metav1.CreateOptions{})

	if err != nil {
		return fmt.Errorf("error while creating job: %v", err)
	}

	err = waitForJobPodStart(ctx, kcmd.Namespace, fmt.Sprintf("%s-%d", taskId, kcmd.JobId))
	if err != nil {
		return fmt.Errorf("error while waiting for job pod to start: %v", err)
	}

	go kcmd.streamLogs()

	watcher, err := client.Watch(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s-%d", taskId, kcmd.JobId)})
	if err != nil {
		return fmt.Errorf("error while watching job: %v", err)
	}
	defer watcher.Stop()

	waitForJobFinish(ctx, watcher)

	return nil
}

func (kcmd KubernetesCommand) streamLogs() {
	clientset, err := getKubernetesClientset()
	if err != nil {
		fmt.Println("Error while getting kubernetes clientset: ", err)
		return
	}

	pods, err := clientset.CoreV1().Pods(kcmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s-%d", kcmd.TaskId, kcmd.JobId)})
	if err != nil {
		fmt.Println("Error while getting pods: ", err)
		return
	}

	pod := pods.Items[0]
	req := clientset.CoreV1().Pods(kcmd.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Follow: true})
	stream, err := req.Stream(context.TODO())

	if err != nil {
		fmt.Println("Error while opening log stream: ", err)
		return
	}

	defer stream.Close()

	_, err = io.Copy(kcmd.Stdout, stream)

	if err != nil {
		fmt.Println("Error while streaming logs: ", err)
		return
	}
}

// Deletes the job running the task.
func (kcmd KubernetesCommand) Stop() error {
	clientset, err := getKubernetesClientset()
	if err != nil {
		return err
	}

	jobName := fmt.Sprintf("%s-%d", kcmd.TaskId, kcmd.JobId)

	backgroundDeletion := metav1.DeletePropagationBackground
	err = clientset.BatchV1().Jobs(kcmd.Namespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{
		PropagationPolicy: &backgroundDeletion,
	})

	if err != nil {
		return fmt.Errorf("error while deleting job: %v", err)
	}

	return nil
}

func waitForJobPodStart(ctx context.Context, namespace string, jobName string) error {
	clientset, err := getKubernetesClientset()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", jobName)})

			if err != nil {
				return err
			}

			if len(pods.Items) > 0 {
				return nil
			}
		}
	}
}

// Waits until the job finishes
func waitForJobFinish(ctx context.Context, watcher watch.Interface) {
	for {
		select {
		case event := <-watcher.ResultChan():
			job := event.Object.(*v1.Job)

			if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
				return
			} else if event.Type == watch.Deleted {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func getKubernetesClientset() (*kubernetes.Clientset, error) {
	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(kubeconfig)
	return clientset, err
}
