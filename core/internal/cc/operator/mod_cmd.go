package operator

import (
	"strings"

	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/agents"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/modules"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/internal/live"
	"github.com/jm33-m0/emp3r0r/core/lib/cli"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"github.com/spf13/cobra"
)

// listModOptionsTable list currently available options for `set`, in a table
func listModOptionsTable() {
	if live.ActiveModule == nil {
		logging.Warningf("No module selected")
		return
	}
	agent := agents.MustGetActiveAgent()
	shortName := "none"
	if agent != nil {
		shortName = strings.Split(agent.Tag, "-agent")[0]
	}

	// build table rows
	rows := [][]string{}
	_, ok := def.Modules[live.ActiveModule.Name]
	if !ok {
		logging.Errorf("Module %s not found", live.ActiveModule.Name)
		return
	}
	rows = append(rows, []string{"module", "Selected module", live.ActiveModule.Name})
	rows = append(rows, []string{"target", "Selected target", shortName})
	for opt_name, opt_obj := range live.ActiveModule.Options {
		help := "N/A"
		if opt_obj == nil {
			continue
		}
		help = opt_obj.Desc
		val := ""
		currentOpt, ok := live.ActiveModule.Options[opt_name]
		if ok {
			val = currentOpt.Val
		}

		rows = append(rows,
			[]string{
				util.SplitLongLine(opt_name, 50),
				util.SplitLongLine(help, 50),
				util.SplitLongLine(val, 50),
			})
	}

	// reuse BuildTable helper
	tableStr := cli.BuildTable([]string{"Option", "Help", "Value"}, rows)
	cli.AdaptiveTable(tableStr)
	logging.Printf("\n%s", tableStr)
}

func cmdModuleRun(_ *cobra.Command, _ []string) {
	if live.ActiveModule == nil {
		logging.Errorf("No module selected")
		return
	}
	// Send command to module
	modules.ModuleRun()
}

func cmdSetOptVal(cmd *cobra.Command, args []string) {
	if live.ActiveModule == nil {
		logging.Errorf("No module selected")
		return
	}
	opt := args[0]
	val := args[1]

	// hand to SetOption helper
	live.SetOption(opt, val)

	listModOptionsTable()
}

func cmdSetActiveModule(cmd *cobra.Command, args []string) {
	// Set active module
	modules.SetActiveModule(args[0])
	listModOptionsTable()
}

func cmdListModules(_ *cobra.Command, _ []string) {
	// table output
	rows := [][]string{}
	for _, mod := range def.Modules {
		rows = append(rows, []string{mod.Name, mod.Comment})
	}
	tableStr := cli.BuildTable([]string{"Module", "Description"}, rows)
	cli.AdaptiveTable(tableStr)
	logging.Printf("\n%s", tableStr)
}

func cmdSearchModule(cmd *cobra.Command, args []string) {
	results := modules.ModuleSearch(args[0])
	row := [][]string{}
	for _, mod := range results {
		row = append(row, []string{mod.Name, mod.Comment})
	}
	tableStr := cli.BuildTable([]string{"Module", "Description"}, row)
	cli.AdaptiveTable(tableStr)
	logging.Printf("\n%s", tableStr)
}

func cmdModuleListOptions(_ *cobra.Command, _ []string) {
	listModOptionsTable()
}
