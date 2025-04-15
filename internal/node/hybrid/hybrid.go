package hybrid

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"go.uber.org/zap"
	"k8s.io/utils/strings/slices"

	"github.com/aws/eks-hybrid/internal/api"
	"github.com/aws/eks-hybrid/internal/daemon"
	"github.com/aws/eks-hybrid/internal/nodeprovider"
	"github.com/aws/eks-hybrid/internal/validation"
)

const (
	nodeIpValidation      = "node-ip-validation"
	kubeletCertValidation = "kubelet-cert-validation"
)

// ValidationRunner is an interface for running validations.
type ValidationRunner[O validation.Validatable[O]] interface {
	Run(ctx context.Context, obj O, validations ...validation.Validation[O]) error
}

type HybridNodeProvider struct {
	nodeConfig    *api.NodeConfig
	validator     func(config *api.NodeConfig) error
	awsConfig     *aws.Config
	daemonManager daemon.DaemonManager
	logger        *zap.Logger
	cluster       *types.Cluster
	skipPhases    []string
	network       Network
	// installRoot is optionally the root directory of the installation
	// If not provided, the cert
	installRoot  string
	singleRunner ValidationRunner[*api.NodeConfig]
}

type NodeProviderOpt func(*HybridNodeProvider)

func NewHybridNodeProvider(nodeConfig *api.NodeConfig, skipPhases []string, logger *zap.Logger, opts ...NodeProviderOpt) (nodeprovider.NodeProvider, error) {
	var skipValidations []string
	for _, phase := range skipPhases {
		skipValidations = append(skipValidations, strings.TrimSuffix(phase, "-validation"))
	}

	np := &HybridNodeProvider{
		nodeConfig: nodeConfig,
		logger:     logger,
		skipPhases: skipPhases,
		network:    &defaultKubeletNetwork{},
		singleRunner: validation.NewSingleRunner[*api.NodeConfig](
			validation.NewZapInformer(logger),
			validation.WithSingleRunnerSkipValidations(skipValidations...),
		),
	}
	np.withHybridValidators()
	if err := np.withDaemonManager(); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(np)
	}

	return np, nil
}

func WithAWSConfig(config *aws.Config) NodeProviderOpt {
	return func(hnp *HybridNodeProvider) {
		hnp.awsConfig = config
	}
}

// WithCluster adds an EKS cluster to the HybridNodeProvider for testing purposes.
func WithCluster(cluster *types.Cluster) NodeProviderOpt {
	return func(hnp *HybridNodeProvider) {
		hnp.cluster = cluster
	}
}

// WithNetwork adds network util functions to the HybridNodeProvider for testing purposes.
func WithNetwork(network Network) NodeProviderOpt {
	return func(hnp *HybridNodeProvider) {
		hnp.network = network
	}
}

// WithInstallRoot sets the root directory for installation paths
func WithInstallRoot(root string) NodeProviderOpt {
	return func(hnp *HybridNodeProvider) {
		hnp.installRoot = root
	}
}

// WithSingleRunner sets the single runner for the HybridNodeProvider.
func WithSingleRunner(runner ValidationRunner[*api.NodeConfig]) NodeProviderOpt {
	return func(hnp *HybridNodeProvider) {
		hnp.singleRunner = runner
	}
}

func (hnp *HybridNodeProvider) GetNodeConfig() *api.NodeConfig {
	return hnp.nodeConfig
}

func (hnp *HybridNodeProvider) Logger() *zap.Logger {
	return hnp.logger
}

func (hnp *HybridNodeProvider) Validate() error {
	if !slices.Contains(hnp.skipPhases, nodeIpValidation) {
		if err := hnp.ValidateNodeIP(); err != nil {
			return err
		}
	}

	if !slices.Contains(hnp.skipPhases, kubeletCertValidation) {
		if err := ValidateKubeletCert(hnp.logger, hnp.installRoot, hnp.nodeConfig.Spec.Cluster.CertificateAuthority); err != nil {
			return err
		}
	}

	return nil
}

func (hnp *HybridNodeProvider) Cleanup() error {
	hnp.daemonManager.Close()
	return nil
}

// getCluster retrieves the Cluster object or makes a DescribeCluster call to the EKS API and caches the result if not already present
func (hnp *HybridNodeProvider) getCluster(ctx context.Context) (*types.Cluster, error) {
	if hnp.cluster != nil {
		return hnp.cluster, nil
	}

	cluster, err := readCluster(ctx, *hnp.awsConfig, hnp.nodeConfig)
	if err != nil {
		return nil, err
	}
	hnp.cluster = cluster

	return cluster, nil
}
