package sampleyaml

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	. "github.com/ray-project/kuberay/ray-operator/test/support"
)

func GetSampleYAMLDir(t Test) string {
	t.T().Helper()
	_, b, _, _ := runtime.Caller(0)
	sampleYAMLDir := filepath.Join(filepath.Dir(b), "../../config/samples")
	info, err := os.Stat(sampleYAMLDir)
	assert.NoError(t.T(), err)
	assert.True(t.T(), info.IsDir())
	return sampleYAMLDir
}

func IsPodRunningAndReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func AllPodsRunningAndReady(pods []corev1.Pod) bool {
	for _, pod := range pods {
		if !IsPodRunningAndReady(&pod) {
			return false
		}
	}
	return true
}

func SubmitJobsToAllPods(t Test, rayCluster *rayv1.RayCluster) func(Gomega) {
	return func(g Gomega) {
		pods, err := GetAllPods(t, rayCluster)
		g.Expect(err).NotTo(HaveOccurred())
		cmd := []string{
			"python",
			"-c",
			"import ray; ray.init(); print(ray.cluster_resources())",
		}
		for _, pod := range pods {
			for _, container := range pod.Spec.Containers {
				ExecPodCmd(t, &pod, container.Name, cmd)
			}
		}
	}
}

func getApps(rayService *rayv1.RayService) map[string]rayv1.AppStatus {
	apps := make(map[string]rayv1.AppStatus)
	for k, v := range rayService.Status.ActiveServiceStatus.Applications {
		apps[k] = v
	}
	return apps
}

func AllAppsRunning(rayService *rayv1.RayService) bool {
	appStatuses := getApps(rayService)
	if len(appStatuses) == 0 {
		return false
	}

	for _, appStatus := range appStatuses {
		if appStatus.Status != rayv1.ApplicationStatusEnum.RUNNING {
			return false
		}
	}
	return true
}

func QueryDashboardGetAppStatus(t Test, rayCluster *rayv1.RayCluster) func(Gomega) {
	return func(g Gomega) {
		rayDashboardClient := &utils.RayDashboardClient{}
		pod, err := GetHeadPod(t, rayCluster)
		g.Expect(err).ToNot(HaveOccurred())

		localPort := 8265
		remotePort := 8265
		stopChan, err := SetupPortForward(t, pod.Name, pod.Namespace, localPort, remotePort)
		defer close(stopChan)

		g.Expect(err).ToNot(HaveOccurred())
		url := fmt.Sprintf("127.0.0.1:%d", localPort)

		err = rayDashboardClient.InitClient(t.Ctx(), url, rayCluster)
		g.Expect(err).ToNot(HaveOccurred())
		serveDetails, err := rayDashboardClient.GetServeDetails(t.Ctx())
		g.Expect(err).ToNot(HaveOccurred())

		for _, value := range serveDetails.Applications {
			g.Expect(value.ServeApplicationStatus.Status).To(Equal(rayv1.ApplicationStatusEnum.RUNNING))
		}
	}
}