package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jessesomerville/ephemeral-iam/cmd/eiam/cmd/options"
	util "github.com/jessesomerville/ephemeral-iam/cmd/eiam/internal/eiamutil"
	"github.com/jessesomerville/ephemeral-iam/cmd/eiam/internal/gcpclient"
)

var (
	gcloudCmdArgs   []string
	gcloudCmdConfig options.CmdConfig
)

func newCmdGcloud() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcloud [GCLOUD_ARGS]",
		Short: "Run a gcloud command with the permissions of the specified service account",
		Long: dedent.Dedent(`
			The "gcloud" command runs the provided gcloud command with the permissions of the specified
			service account. Output from the gcloud command is able to be piped into other commands.`),
		Example: dedent.Dedent(`
			eiam gcloud compute instances list --format=json \
			--service-account-email example@my-project.iam.gserviceaccount.com \
			--reason "Debugging for (JIRA-1234)"
			
			eiam gcloud compute instances list --format=json \
			-s example@my-project.iam.gserviceaccount.com -r "example" \
			| jq`),
		Args:               cobra.ArbitraryArgs,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		PreRun: func(cmd *cobra.Command, args []string) {
			cmd.Flags().VisitAll(options.CheckRequired)

			gcloudCmdArgs = util.ExtractUnknownArgs(cmd.Flags(), os.Args)
			util.CheckError(util.FormatReason(&gcloudCmdConfig.Reason))

			if !options.YesOption {
				util.Confirm(map[string]string{
					"Project":         gcloudCmdConfig.Project,
					"Service Account": gcloudCmdConfig.ServiceAccountEmail,
					"Reason":          gcloudCmdConfig.Reason,
					"Command":         fmt.Sprintf("gcloud %s", strings.Join(gcloudCmdArgs, " ")),
				})
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGcloudCommand()
		},
	}

	options.AddServiceAccountEmailFlag(cmd.Flags(), &gcloudCmdConfig.ServiceAccountEmail, true)
	options.AddReasonFlag(cmd.Flags(), &gcloudCmdConfig.Reason, true)
	options.AddProjectFlag(cmd.Flags(), &gcloudCmdConfig.Project)

	return cmd
}

func runGcloudCommand() error {
	hasAccess, err := gcpclient.CanImpersonate(
		gcloudCmdConfig.Project,
		gcloudCmdConfig.ServiceAccountEmail,
		gcloudCmdConfig.Reason,
	)
	if err != nil {
		return err
	} else if !hasAccess {
		util.Logger.Fatalln("You do not have access to impersonate this service account")
	}

	// gcloud reads the CLOUDSDK_CORE_REQUEST_REASON environment variable
	// and sets the X-Goog-Request-Reason header in API requests to its value
	reasonHeader := fmt.Sprintf("CLOUDSDK_CORE_REQUEST_REASON=%s", gcloudCmdConfig.Reason)

	// There has to be a better way to do this...
	util.Logger.Infof("Running: [gcloud %s]\n\n", strings.Join(gcloudCmdArgs, " "))
	gcloudCmdArgs = append(gcloudCmdArgs, "--impersonate-service-account", gcloudCmdConfig.ServiceAccountEmail, "--verbosity=error")
	c := exec.Command(viper.GetString("binarypaths.gcloud"), gcloudCmdArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), reasonHeader)

	if err := c.Run(); err != nil {
		fullCmd := fmt.Sprintf("gcloud %s", strings.Join(gcloudCmdArgs, " "))
		return fmt.Errorf("Error: %v for command [%s]", err, fullCmd)
	}
	return nil
}
