package options

import (
	"github.com/spf13/pflag"

	util "github.com/jessesomerville/ephemeral-iam/internal/eiamutil"
	errorsutil "github.com/jessesomerville/ephemeral-iam/internal/errors"
	"github.com/jessesomerville/ephemeral-iam/internal/gcpclient"
)

// Flag annotation strings
const (
	RequiredAnnotation = "eiam_required_flag"
)

// YesOption designates whether to prompt for confirmation or not
var YesOption = false

// Flag names and shorthands
var (
	ProjectFlag             = flagName{"project", "p"}
	ReasonFlag              = flagName{"reason", "R"}
	RegionFlag              = flagName{"region", "r"}
	ServiceAccountEmailFlag = flagName{"service-account-email", "s"}
	YesFlag                 = flagName{"yes", "y"}
	ZoneFlag                = flagName{"zone", "z"}
)

type flagName struct {
	Name      string
	Shorthand string
}

// CmdConfig holds the values passed to a command
type CmdConfig struct {
	ComputeInstance     string
	Project             string
	PubSubTopic         string
	Reason              string
	Region              string
	ServiceAccountEmail string
	StorageBucket       string
	Zone                string
}

// AddPersistentFlags add persistent flags to the root command
func AddPersistentFlags(fs *pflag.FlagSet) {
	fs.BoolVarP(&YesOption, YesFlag.Name, YesFlag.Shorthand, YesOption, "Assume 'yes' to all prompts")
}

// AddProjectFlag adds the --project/-p flag to the command
func AddProjectFlag(fs *pflag.FlagSet, project *string) {
	defaultVal, err := gcpclient.GetCurrentProject()
	errorsutil.CheckError(err)

	fs.StringVarP(project, ProjectFlag.Name, ProjectFlag.Shorthand, defaultVal, "The GCP project. Inherits from the active gcloud config by default")
	if defaultVal == "" {
		if err := fs.SetAnnotation(ProjectFlag.Name, RequiredAnnotation, []string{"true"}); err != nil {
			util.Logger.Fatalf("failed to set required annotation on flag: %v", err)
		}
	}
}

// AddRegionFlag adds the --region/-r flag to the command
func AddRegionFlag(fs *pflag.FlagSet, region *string, required bool) {
	defaultVal, err := gcpclient.GetCurrentRegion()
	errorsutil.CheckError(err)

	fs.StringVarP(region, RegionFlag.Name, RegionFlag.Shorthand, defaultVal, "The GCP region. Inherits from the active gcloud config by default")
	if required {
		if err := fs.SetAnnotation(RegionFlag.Name, RequiredAnnotation, []string{"true"}); err != nil {
			util.Logger.Fatalf("failed to set required annotation on flag: %v", err)
		}
	}
}

// AddZoneFlag adds the --zone/-z flag to the command
func AddZoneFlag(fs *pflag.FlagSet, zone *string, required bool) {
	defaultVal, err := gcpclient.GetCurrentZone()
	errorsutil.CheckError(err)

	fs.StringVarP(zone, ZoneFlag.Name, ZoneFlag.Shorthand, defaultVal, "The GCP zone. Inherits from the active gcloud config by default")
	if required {
		if err := fs.SetAnnotation(ZoneFlag.Name, RequiredAnnotation, []string{"true"}); err != nil {
			util.Logger.Fatalf("failed to set required annotation on flag: %v", err)
		}
	}
}

// AddServiceAccountEmailFlag adds the --service-account-email/-s flag
func AddServiceAccountEmailFlag(fs *pflag.FlagSet, serviceAccountEmail *string, required bool) {
	fs.StringVarP(serviceAccountEmail, ServiceAccountEmailFlag.Name, ServiceAccountEmailFlag.Shorthand, "", "The email address for the service account")
	if required {
		if err := fs.SetAnnotation(ServiceAccountEmailFlag.Name, RequiredAnnotation, []string{"true"}); err != nil {
			util.Logger.Fatalf("failed to set required annotation on flag: %v", err)
		}
	}
}

// AddReasonFlag adds the --reason/-R flag
func AddReasonFlag(fs *pflag.FlagSet, reason *string, required bool) {
	fs.StringVarP(reason, ReasonFlag.Name, ReasonFlag.Shorthand, "", "A detailed rationale for assuming higher permissions")
	if required {
		if err := fs.SetAnnotation(ReasonFlag.Name, RequiredAnnotation, []string{"true"}); err != nil {
			util.Logger.Fatalf("failed to set required annotation on flag: %v", err)
		}
	}
}

// CheckRequired ensures that a command's required flags have been set
func CheckRequired(flag *pflag.Flag) {
	for annot, val := range flag.Annotations {
		if annot == RequiredAnnotation && val[0] == "true" {
			if flag.Value.String() == "" {
				util.Logger.Fatalf("Missing required flag: %s", flag.Name)
			}
		}
	}
}
