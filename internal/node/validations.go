package node

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/aws/eks-hybrid/internal/kubelet"
	k8s "github.com/aws/eks-hybrid/internal/kubernetes"
	"github.com/aws/eks-hybrid/internal/node/hybrid"
)

const defaultStaticPodManifestPath = "/etc/kubernetes/manifest"

func IsUnscheduled(ctx context.Context) error {
	node, err := getCurrentNode(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !node.Spec.Unschedulable {
		return fmt.Errorf("node is schedulable")
	}
	return nil
}

func IsDrained(ctx context.Context) (bool, error) {
	nodeName, err := kubelet.GetNodeName()
	if err != nil {
		return false, errors.Wrap(err, "getting node name from kubelet")
	}

	clientset, err := hybrid.BuildKubeClient()
	if err != nil {
		return false, errors.Wrap(err, "failed to create kubernetes client")
	}

	pods, err := GetPodsOnNode(ctx, nodeName, clientset)
	if err != nil {
		return false, errors.Wrapf(err, "getting pods for node %s", nodeName)
	}

	return isDrained(pods)
}

func isDrained(pods []v1.Pod) (bool, error) {
	for _, filter := range getDrainedPodFilters() {
		var err error
		pods, err = filter(pods)
		if err != nil {
			return false, errors.Wrap(err, "running filter on pods")
		}
	}

	return len(pods) == 0, nil
}

func IsInitialized(ctx context.Context) error {
	_, err := getCurrentNode(ctx)
	if err != nil {
		return err
	}
	return nil
}

func getCurrentNode(ctx context.Context) (*v1.Node, error) {
	nodeName, err := kubelet.GetNodeName()
	if err != nil {
		return nil, err
	}

	clientset, err := hybrid.BuildKubeClient()
	if err != nil {
		return nil, err
	}

	return k8s.GetRetry(ctx, clientset.CoreV1().Nodes(), nodeName)
}
