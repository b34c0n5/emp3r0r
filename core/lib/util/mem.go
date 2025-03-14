package util

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/lib/crypto"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
)

// ExtractData extract embedded data from args[0] or process memory
func ExtractData() (data []byte, err error) {
	data, err = extractFromAgentConfig()
	if err != nil {
		err = fmt.Errorf("extract data from emp3r0r_def.AgentConfig: %v", err)
		return
	}

	if len(data) <= 0 {
		err = fmt.Errorf("no data extracted")
	}

	return
}

func extractFromExecutable() ([]byte, error) {
	data, err := DigEmbeddedDataFromExe()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func extractFromMemory() ([]byte, error) {
	data, err := DigEmbededDataFromMem()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func extractFromAgentConfig() ([]byte, error) {
	// Get raw config bytes and strip trailing null bytes
	enc_config := bytes.Trim(def.AgentConfig[:], "\x00")

	// decrypt and verify
	return VerifyConfigData(enc_config)
}

func VerifyConfigData(data []byte) (jsonData []byte, err error) {
	// decrypt attached JSON file
	jsonData, err = crypto.AES_GCM_Decrypt([]byte(def.MagicString), data)
	if err != nil {
		err = fmt.Errorf("decrypt config JSON failed (%v), invalid config data?", err)
		return
	}

	return
}

// GetProcessExe dump executable of target process
func GetProcessExe(pid int) (exe_data []byte, err error) {
	process_exe_file := fmt.Sprintf("/proc/%d/exe", pid)
	if runtime.GOOS == "windows" {
		process_exe_file = os.Args[0]
	}
	exe_data, err = os.ReadFile(process_exe_file)

	return
}

// DigEmbededDataFromFile search args[0] file content for data embeded between two separators
// separator is MagicString*3
func DigEmbeddedDataFromExe() ([]byte, error) {
	wholeStub, err := GetProcessExe(os.Getpid())
	logging.Debugf("Read %d bytes from process executable", len(wholeStub))
	if err != nil {
		return nil, err
	}

	return DigEmbeddedData(wholeStub, 0)
}

// DigEmbeddedData search for embedded data in given []byte buffer
// base is the starting address of the buffer (memory region), will be ignored if 0
func DigEmbeddedData(data []byte, base int64) (embedded_data []byte, err error) {
	// OneTimeMagicBytes is 16 bytes long random data,
	// generated by CC per session (delete ~/.emp3r0r to reset)
	// we use it to locate the embedded data
	magic_str := []byte(def.MagicString) // used to be def.OneTimeMagicBytes
	logging.Debugf("Digging with magic string '%x' (%d bytes)", magic_str, len(magic_str))
	sep := bytes.Repeat(magic_str, 2)

	if !bytes.Contains(data, sep) {
		err = fmt.Errorf("cannot locate magic string '%x' in %d bytes of given data",
			magic_str, len(data))
		return
	}

	// locate embedded_data
	split := bytes.Split(data, sep)
	if len(split) < 2 {
		err = fmt.Errorf("cannot locate embeded data from %d of given data", len(data))
		return
	}
	embedded_data = split[1]
	if len(embedded_data) <= 0 {
		err = fmt.Errorf("digged nothing from %d of given data", len(data))
		return
	}

	// found and verify
	embedded_data, err = VerifyConfigData(embedded_data)
	if err != nil {
		err = fmt.Errorf("verify config data: %v", err)
		return
	}

	// confirm
	logging.Debugf("Digged %d config bytes from %d bytes of given data at (0x%x)", len(embedded_data), len(data), base)
	return
}

// DigEmbededDataFromMem search process memory for data embeded between two separators
// separator is MagicString*3
func DigEmbededDataFromMem() (data []byte, err error) {
	mem_regions, err := DumpSelfMem()
	if err != nil {
		err = fmt.Errorf("cannot dump self memory: %v", err)
		return
	}

	for base, mem_region := range mem_regions {
		data, err = DigEmbeddedData(mem_region, base)
		if err != nil {
			logging.Debugf("Nothing in memory region %d (%d bytes): %v", base, len(mem_region), err)
			continue
		}
		break
	}
	if len(data) <= 0 {
		return nil, fmt.Errorf("no data found in memory")
	}

	return
}

// DumpSelfMem dump all mapped memory regions of current process
func DumpSelfMem() (map[int64][]byte, error) {
	return DumpCurrentProcMem()
}
