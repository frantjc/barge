package barge

import (
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/cli"
)

var (
	helm = cli.New()
)

func AddFlags(fs *pflag.FlagSet) {
	helm.AddFlags(fs)
}

func HelmSettings() *cli.EnvSettings {
	return helm
}
