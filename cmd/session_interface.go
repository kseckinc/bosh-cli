package cmd

import (
	cmdconf "github.com/cloudfoundry/bosh-cli/cmd/config"
	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
)

//counterfeiter:generate . SessionContext

type SessionContext interface {
	Environment() string
	CACert() string
	Config() cmdconf.Config
	Credentials() cmdconf.Creds

	Deployment() string
}

//counterfeiter:generate . Session

type Session interface {
	Environment() string
	Credentials() cmdconf.Creds

	UAA() (boshuaa.UAA, error)

	Director() (boshdir.Director, error)
	AnonymousDirector() (boshdir.Director, error)

	Deployment() (boshdir.Deployment, error)
}
