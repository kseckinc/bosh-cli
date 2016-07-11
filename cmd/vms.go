package cmd

import (
	"fmt"

	boshdir "github.com/cloudfoundry/bosh-init/director"
	boshui "github.com/cloudfoundry/bosh-init/ui"
	boshtbl "github.com/cloudfoundry/bosh-init/ui/table"
)

type VMsCmd struct {
	ui       boshui.UI
	director boshdir.Director
}

func NewVMsCmd(ui boshui.UI, director boshdir.Director) VMsCmd {
	return VMsCmd{ui: ui, director: director}
}

func (c VMsCmd) Run(opts VMsOpts) error {
	deployments, err := c.director.Deployments()
	if err != nil {
		return err
	}

	instTable := InstanceTable{
		// VMs command should always show VM specifics
		VMDetails: true,

		Details: opts.Details,
		DNS:     opts.DNS,
		Vitals:  opts.Vitals,
	}

	for _, dep := range deployments {
		vmInfos, err := dep.VMInfos()
		if err != nil {
			return err
		}

		table := boshtbl.Table{
			Title: fmt.Sprintf("Deployment '%s'", dep.Name()),

			Content: "vms",

			HeaderVals: instTable.AsValues(instTable.Header()),

			SortBy: []boshtbl.ColumnSort{{Column: 0, Asc: true}},

			Notes: []string{"(*) Bootstrap node"},
		}

		for _, info := range vmInfos {
			row := instTable.AsValues(instTable.ForVMInfo(info))

			table.Rows = append(table.Rows, row)
		}

		c.ui.PrintTable(table)
	}

	return nil
}