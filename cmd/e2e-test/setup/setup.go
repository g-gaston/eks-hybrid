package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/aws/eks-hybrid/internal/cli"
	"github.com/aws/eks-hybrid/test/e2e"
	"github.com/aws/eks-hybrid/test/e2e/cluster"
)

type command struct {
	flaggy         *flaggy.Subcommand
	configFilePath string
}

func NewCommand() cli.Command {
	cmd := command{}

	setupCmd := flaggy.NewSubcommand("setup")
	setupCmd.Description = "Create the E2E test infrastructure"
	setupCmd.AdditionalHelpPrepend = "This command will run the setup infrastructure for running E2E tests"

	setupCmd.String(&cmd.configFilePath, "s", "setup-config-path", "Path to setup config file")

	cmd.flaggy = setupCmd

	return &cmd
}

func (c *command) Flaggy() *flaggy.Subcommand {
	return c.flaggy
}

func (s *command) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	ctx := context.Background()
	file, err := os.ReadFile(s.configFilePath)
	if err != nil {
		return fmt.Errorf("failed to open configuration file: %v", err)
	}

	testResources := cluster.TestResources{}

	if err = yaml.Unmarshal(file, &testResources); err != nil {
		return fmt.Errorf("unmarshaling test infra configuration: %w", err)
	}

	aws, err := config.LoadDefaultConfig(ctx, config.WithRegion(testResources.ClusterRegion))
	if err != nil {
		return fmt.Errorf("reading AWS configuration: %w", err)
	}

	logger := e2e.NewLogger()
	create := cluster.NewCreate(aws, logger)

	logger.Info("Creating cluster infrastructure for E2E tests...")
	if err := create.Run(ctx, testResources); err != nil {
		return fmt.Errorf("creating E2E test infrastructure: %w", err)
	}

	fmt.Println("E2E test infrastructure setup completed successfully!")
	return nil
}
