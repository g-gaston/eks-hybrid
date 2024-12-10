package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/eks-hybrid/test/e2e/constants"
	"github.com/aws/eks-hybrid/test/e2e/errors"
	"github.com/go-logr/logr"
)

const (
	createClusterTimeout = 15 * time.Minute
)

type hybridCluster struct {
	Name              string
	Region            string
	KubernetesVersion string
	SecurityGroup     string
	SubnetIDs         []string
	Role              string
	HybridNetwork     NetworkConfig
}

func (h *hybridCluster) create(ctx context.Context, client *eks.Client, logger logr.Logger) error {
	hybridCluster := &eks.CreateClusterInput{
		Name:    aws.String(h.Name),
		Version: aws.String(h.KubernetesVersion),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds:        h.SubnetIDs,
			SecurityGroupIds: []string{h.SecurityGroup},
		},
		RoleArn: aws.String(h.Role),
		Tags: map[string]string{
			constants.TestClusterTagKey: h.Name,
		},
		AccessConfig: &types.CreateAccessConfigRequest{
			AuthenticationMode: types.AuthenticationModeApiAndConfigMap,
		},
		RemoteNetworkConfig: &types.RemoteNetworkConfigRequest{
			RemoteNodeNetworks: []types.RemoteNodeNetwork{
				{
					Cidrs: []string{h.HybridNetwork.VpcCidr},
				},
			},
			RemotePodNetworks: []types.RemotePodNetwork{
				{
					Cidrs: []string{h.HybridNetwork.PodCidr},
				},
			},
		},
	}
	clusterOutput, err := client.CreateCluster(ctx, hybridCluster)
	if err != nil && !errors.IsType(err, &types.ResourceInUseException{}) {
		return fmt.Errorf("creating EKS hybrid cluster: %w", err)
	}

	logger.Info("Waiting for cluster to be active", "cluster", h.Name)
	if err := waitForActiveCluster(ctx, client, h.Name); err != nil {
		return err
	}

	if clusterOutput.Cluster != nil {
		logger.Info("Successfully started EKS hybrid cluster", "output", awsutil.Prettify(clusterOutput))
	}

	return nil
}

// waitForActiveCluster waits until the cluster is in the 'ACTIVE' state.
func waitForActiveCluster(ctx context.Context, client *eks.Client, clusterName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), createClusterTimeout)
	defer cancel()

	return waitForCluster(ctx, client, clusterName, func(output *eks.DescribeClusterOutput, err error) (bool, error) {
		if err != nil {
			return false, fmt.Errorf("describing cluster %s: %w", clusterName, err)
		}

		switch output.Cluster.Status {
		case types.ClusterStatusActive:
			return true, nil
		case types.ClusterStatusFailed:
			return false, fmt.Errorf("cluster %s creation failed", clusterName)
		default:
			return false, nil
		}
	})
}

func (h *hybridCluster) UpdateKubeconfig(kubeconfig string) error {
	cmd := exec.Command("aws", "eks", "update-kubeconfig", "--name", h.Name, "--region", h.Region, "--kubeconfig", kubeconfig)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForCluster(ctx context.Context, client *eks.Client, clusterName string, check func(*eks.DescribeClusterOutput, error) (bool, error)) error {
	statusCh := make(chan bool)
	errCh := make(chan error)

	go func(ctx context.Context) {
		defer close(statusCh)
		defer close(errCh)
		for {
			describeInput := &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			}
			done, err := check(client.DescribeCluster(ctx, describeInput))
			if err != nil {
				errCh <- err
				return
			}

			if done {
				return
			}

			select {
			case <-ctx.Done(): // Check if the context is done (timeout/canceled)
				errCh <- fmt.Errorf("context canceled or timed out while waiting for cluster %s: %v", clusterName, ctx.Err())
				return
			case <-time.After(30 * time.Second):
			}
		}
	}(ctx)

	// Wait for the cluster to be deleted or for the timeout to expire
	select {
	case <-statusCh:
		return nil
	case err := <-errCh:
		return err
	}
}
