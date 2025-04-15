package kubelet

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/kubernetes"
	"github.com/aws/eks-hybrid/internal/validation"
)

const KubeletDaemonName = "kubelet"

var _ daemon.Daemon = &kubelet{}

type kubelet struct {
	daemonManager daemon.DaemonManager
	awsConfig     *aws.Config
	nodeConfig    *api.NodeConfig
	// environment variables to write for kubelet
	environment map[string]string
	// kubelet config flags without leading dashes
	flags map[string]string
	// runner performs in-flight validations
	runner ValidationRunner
}

// ValidationRunner runs validations.
type ValidationRunner interface {
	Run(ctx context.Context, obj *api.NodeConfig, validations ...validation.Validation[*api.NodeConfig]) error
}

func NewKubeletDaemon(daemonManager daemon.DaemonManager, cfg *api.NodeConfig, awsConfig *aws.Config, runner ValidationRunner) daemon.Daemon {
	return &kubelet{
		daemonManager: daemonManager,
		nodeConfig:    cfg,
		awsConfig:     awsConfig,
		environment:   make(map[string]string),
		flags:         make(map[string]string),
		runner:        runner,
	}
}

func (k *kubelet) Configure(ctx context.Context) error {
	if err := k.writeKubeletConfig(); err != nil {
		return err
	}
	if err := k.writeKubeconfig(); err != nil {
		return err
	}
	if err := k.writeImageCredentialProviderConfig(); err != nil {
		return err
	}
	if err := writeClusterCaCert(k.nodeConfig.Spec.Cluster.CertificateAuthority); err != nil {
		return err
	}
	if err := k.writeKubeletEnvironment(); err != nil {
		return err
	}

	// At this point we have a valid kubeconfig so we should be able to make an authenticated request
	if err := k.runner.Run(ctx, k.nodeConfig,
		validation.New("k8s-authentication", kubernetes.NewAPIServerValidator(Kubeconfig{}).MakeAuthenticatedRequest),
	); err != nil {
		return err
	}
	return nil
}

func (k *kubelet) EnsureRunning(ctx context.Context) error {
	if err := k.daemonManager.DaemonReload(); err != nil {
		return err
	}
	err := k.daemonManager.EnableDaemon(KubeletDaemonName)
	if err != nil {
		return err
	}
	return k.daemonManager.RestartDaemon(ctx, KubeletDaemonName)
}

func (k *kubelet) PostLaunch() error {
	return nil
}

func (k *kubelet) Stop() error {
	return k.daemonManager.StopDaemon(KubeletDaemonName)
}

func (k *kubelet) Name() string {
	return KubeletDaemonName
}
