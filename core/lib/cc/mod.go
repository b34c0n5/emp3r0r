//go:build linux
// +build linux

package cc

import (
	"fmt"
	"strconv"
	"strings"

	emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// CurrentOption all necessary info of an option
type CurrentOption struct {
	Name string   // like `module`, `target`, `cmd_to_exec`
	Val  string   // the value to use
	Vals []string // possible values
}

var (
	// ModuleDir stores modules
	ModuleDirs []string

	// CurrentMod selected module
	CurrentMod = "<blank>"

	// CurrentTarget selected target
	CurrentTarget *emp3r0r_def.Emp3r0rAgent

	// CurrentModuleOptions currently available options for `set`
	CurrentModuleOptions = make(map[string]*CurrentOption)

	// ShellHelpInfo provide utilities like ps, kill, etc
	// deprecated
	ShellHelpInfo = map[string]string{
		"#ps":   "List processes: `ps`",
		"#kill": "Kill process: `kill <PID>`",
		"#net":  "Show network info",
		"put":   "Put a file from CC to agent: `put <local file> <remote path>`",
		"get":   "Get a file from agent: `get <remote file>`",
	}

	// ModuleHelpers a map of module helpers
	ModuleHelpers = map[string]func(){
		emp3r0r_def.ModCMD_EXEC:     moduleCmd,
		emp3r0r_def.ModSHELL:        moduleShell,
		emp3r0r_def.ModPROXY:        moduleProxy,
		emp3r0r_def.ModPORT_FWD:     modulePortFwd,
		emp3r0r_def.ModLPE_SUGGEST:  moduleLPE,
		emp3r0r_def.ModCLEAN_LOG:    moduleLogCleaner,
		emp3r0r_def.ModPERSISTENCE:  modulePersistence,
		emp3r0r_def.ModVACCINE:      moduleVaccine,
		emp3r0r_def.ModINJECTOR:     moduleInjector,
		emp3r0r_def.ModBring2CC:     moduleBring2CC,
		emp3r0r_def.ModListener:     modListener,
		emp3r0r_def.ModSSHHarvester: module_ssh_harvester,
		emp3r0r_def.ModDownloader:   moduleDownloader,
		emp3r0r_def.ModFileServer:   moduleFileServer,
		emp3r0r_def.ModMemDump:      moduleMemDump,
	}
)

// SetOption set an option to value, `set` command
func SetOption(opt, val string) {
	// set
	CurrentModuleOptions[opt].Val = val
}

// UpdateOptions reads options from modules config, and set default values
func UpdateOptions(modName string) (exist bool) {
	// filter user supplied option
	for mod := range ModuleHelpers {
		if mod == modName {
			exist = true
			break
		}
	}
	if !exist {
		LogError("UpdateOptions: no such module: %s", modName)
		return
	}

	// help us add new Option to Options, if exists, return the *Option
	addIfNotFound := func(key string) *CurrentOption {
		if _, exist := CurrentModuleOptions[key]; !exist {
			CurrentModuleOptions[key] = &CurrentOption{Name: key, Val: "<blank>", Vals: []string{}}
		}
		return CurrentModuleOptions[key]
	}

	// other modules
	modconfig := emp3r0r_def.Modules[modName]
	for optName, option := range modconfig.Options {
		argOpt := addIfNotFound(optName)

		argOpt.Val = option.OptVal
	}
	if strings.ToLower(modconfig.AgentConfig.Exec) != "built-in" {
		download_addr := addIfNotFound("download_addr")
		download_addr.Val = ""
	}

	return
}

// ModuleRun run current module
func ModuleRun(_ *cobra.Command, _ []string) {
	modObj := emp3r0r_def.Modules[CurrentMod]
	if modObj == nil {
		LogError("ModuleRun: module %s not found", strconv.Quote(CurrentMod))
		return
	}
	if CurrentTarget != nil {
		target_os := CurrentTarget.GOOS
		mod_os := strings.ToLower(modObj.Platform)
		if mod_os != "generic" && target_os != mod_os {
			LogError("ModuleRun: module %s does not support %s", strconv.Quote(CurrentMod), target_os)
			return
		}
	}

	// is a target needed?
	if CurrentTarget == nil && !modObj.IsLocal {
		LogError("Target not specified")
		return
	}

	// check if target exists
	if Targets[CurrentTarget] == nil && CurrentTarget != nil {
		LogError("Target (%s) does not exist", CurrentTarget.Tag)
		return
	}

	// run module
	mod := ModuleHelpers[CurrentMod]
	if mod != nil {
		go mod()
	} else {
		LogError("Module %s not found", strconv.Quote(CurrentMod))
	}
}

// ValidateActiveTarget check if current target is set and alive
func ValidateActiveTarget() (target *emp3r0r_def.Emp3r0rAgent) {
	// find target
	target = CurrentTarget
	if target == nil {
		LogDebug("Validate active target: target does not exist")
		return nil
	}

	// write to given target's connection
	tControl := Targets[target]
	if tControl == nil {
		LogDebug("Validate active target: agent control interface not found")
		return nil
	}
	if tControl.Conn == nil {
		LogDebug("Validate active target: agent is not connected")
		return nil
	}

	return
}

// search modules, powered by fuzzysearch
func ModuleSearch(cmd *cobra.Command, args []string) {
	keyword, err := cmd.Flags().GetString("keyword")
	if err != nil {
		LogError("ModuleSearch: %v", err)
		return
	}
	if keyword == "" {
		LogError("ModuleSearch: no keyword provided")
		return
	}
	search_targets := new([]string)
	for name, comment := range ModuleNames {
		*search_targets = append(*search_targets, fmt.Sprintf("%s: %s", name, comment))
	}
	result := fuzzy.Find(keyword, *search_targets)

	// render results
	search_results := make(map[string]string)
	for _, r := range result {
		r_split := strings.Split(r, ": ")
		if len(r_split) == 2 {
			search_results[r_split[0]] = r_split[1]
		}
	}
	CliPrettyPrint("Module", "Comment", &search_results)
}

// listModOptionsTable list currently available options for `set`
func listModOptionsTable(_ *cobra.Command, _ []string) {
	if CurrentMod == "none" {
		LogWarning("No module selected")
		return
	}
	TargetsMutex.RLock()
	defer TargetsMutex.RUnlock()
	opts := make(map[string]string)

	opts["module"] = CurrentMod
	if CurrentTarget != nil {
		_, exist := Targets[CurrentTarget]
		if exist {
			shortName := strings.Split(CurrentTarget.Tag, "-agent")[0]
			opts["target"] = shortName
		} else {
			opts["target"] = "<blank>"
		}
	} else {
		opts["target"] = "<blank>"
	}

	for opt_name, opt := range CurrentModuleOptions {
		if opt != nil {
			opts[opt_name] = opt.Name
		}
	}

	// build table
	tdata := [][]string{}
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	table.SetHeader([]string{"Option", "Help", "Value"})
	table.SetBorder(true)
	table.SetRowLine(true)
	table.SetAutoWrapText(true)
	table.SetColWidth(50)

	// color
	table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})
	table.SetColumnColor(tablewriter.Colors{tablewriter.FgHiBlueColor},
		tablewriter.Colors{tablewriter.FgBlueColor},
		tablewriter.Colors{tablewriter.FgBlueColor})

	// fill table
	module_obj := emp3r0r_def.Modules[CurrentMod]
	if module_obj == nil {
		LogError("Module %s not found", CurrentMod)
		return
	}
	for opt_name, opt_obj := range module_obj.Options {
		help := "N/A"
		if opt_obj == nil {
			continue
		}
		help = opt_obj.OptDesc
		switch opt_name {
		case "module":
			help = "Selected module"
		case "target":
			help = "Selected target"
		}
		val := ""
		currentOpt, ok := CurrentModuleOptions[opt_name]
		if ok {
			val = currentOpt.Val
		}

		tdata = append(tdata,
			[]string{
				util.SplitLongLine(opt_name, 50),
				util.SplitLongLine(help, 50),
				util.SplitLongLine(val, 50),
			})
	}
	table.AppendBulk(tdata)
	table.Render()
	out := tableString.String()
	AdaptiveTable(out)
	LogMsg("\n%s", out)
}

func setOptValCmd(cmd *cobra.Command, args []string) {
	opt := args[0]
	val := args[1]
	// hand to SetOption helper
	SetOption(opt, val)
	listModOptionsTable(cmd, args)
}

func setActiveModule(cmd *cobra.Command, args []string) {
	modName := args[0]
	for mod := range ModuleHelpers {
		if mod == modName {
			CurrentMod = modName
			for k := range CurrentModuleOptions {
				delete(CurrentModuleOptions, k)
			}
			UpdateOptions(CurrentMod)
			LogInfo("Using module %s", strconv.Quote(CurrentMod))
			ModuleDetails(CurrentMod)
			mod, exists := emp3r0r_def.Modules[CurrentMod]
			if exists {
				LogMsg("%s", mod.Comment)
			}
			listModOptionsTable(cmd, args)

			return
		}
	}
	LogError("No such module: %s", strconv.Quote(modName))
}
