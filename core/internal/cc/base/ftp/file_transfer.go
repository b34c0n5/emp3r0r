package ftp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/agents"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/network"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/internal/live"
	"github.com/jm33-m0/emp3r0r/core/lib/crypto"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
)

// StatFile Get stat info of a file on agent
func StatFile(filepath string, a *def.Emp3r0rAgent) (fi *util.FileStat, err error) {
	cmd_id := uuid.NewString()
	cmd := fmt.Sprintf("%s --path '%s'", def.C2CmdStat, filepath)
	err = agents.SendCmd(cmd, cmd_id, a)
	if err != nil {
		return
	}
	var fileinfo util.FileStat

	defer func() {
		live.CmdResultsMutex.Lock()
		delete(live.CmdResults, cmd_id)
		live.CmdResultsMutex.Unlock()
	}()

	for {
		time.Sleep(100 * time.Millisecond)
		res, exists := live.CmdResults[cmd_id]
		if exists {
			err = json.Unmarshal([]byte(res), &fileinfo)
			if err != nil {
				return
			}
			fi = &fileinfo
			break
		}
	}

	return
}

// PutFile put file to agent
func PutFile(lpath, rpath string, a *def.Emp3r0rAgent) error {
	// file sha256sum
	logging.Infof("Calculating sha256sum of '%s'", lpath)
	sum := crypto.SHA256SumFile(lpath)
	// file size
	size := util.FileSize(lpath)
	sizemB := float32(size) / 1024 / 1024
	logging.Printf("\nPutFile:\nUploading '%s' to\n'%s' "+
		"on %s, agent [%d]\n"+
		"size: %d bytes (%.2fMB)\n"+
		"sha256sum: %s",
		lpath, rpath,
		a.From, live.AgentControlMap[a].Index,
		size, sizemB,
		sum,
	)

	// move file to wwwroot, then move it back when we are done with it
	logging.Infof("Copying %s to %s", lpath, live.WWWRoot+util.FileBaseName(lpath))
	err := util.Copy(lpath, live.WWWRoot+util.FileBaseName(lpath))
	if err != nil {
		return fmt.Errorf("copy %s to %s: %v", lpath, live.WWWRoot+util.FileBaseName(lpath), err)
	}

	// send cmd
	cmd := fmt.Sprintf("put --file '%s' --path '%s' --checksum %s --size %d", lpath, rpath, sum, size)
	err = agents.SendCmd(cmd, "", a)
	if err != nil {
		return fmt.Errorf("PutFile send command: %v", err)
	}
	logging.Infof("Waiting for response from agent %s", a.Tag)
	return nil
}

// GenerateGetFilePaths generates paths and filenames for GetFile
func GenerateGetFilePaths(file_path string) (write_dir, save_to_file, tempname, lock string) {
	file_path = filepath.Clean(file_path)
	write_dir = fmt.Sprintf("%s%s", live.FileGetDir, filepath.Dir(file_path))
	save_to_file = fmt.Sprintf("%s/%s", write_dir, util.FileBaseName(file_path))
	tempname = save_to_file + ".downloading"
	lock = save_to_file + ".lock"
	return
}

// GetFile get file from agent
func GetFile(file_path string, agent *def.Emp3r0rAgent) (ftpSh *network.StreamHandler, err error) {
	logging.Infof("Waiting for response from agent %s", agent.Tag)

	write_dir, save_to_file, tempname, lock := GenerateGetFilePaths(file_path)
	logging.Debugf("Get file: %s, save to: %s, tempname: %s, lock: %s", file_path, save_to_file, tempname, lock)

	// create directories
	if !util.IsDirExist(write_dir) {
		logging.Infof("Creating directory: %s", strconv.Quote(write_dir))
		err = os.MkdirAll(write_dir, 0o700)
		if err != nil {
			err = fmt.Errorf("GetFile mkdir %s: %v", write_dir, err)
			return
		}
	}

	// is this file already being downloaded?
	if util.IsExist(lock) {
		err = fmt.Errorf("%s is already being downloaded", save_to_file)
		return
	}

	// stat target file, know its size, and allocate the file on disk
	fi, err := StatFile(file_path, agent)
	if err != nil {
		err = fmt.Errorf("GetFile: failed to stat %s: %v", file_path, err)
		return
	}
	fileinfo := *fi
	filesize := fileinfo.Size
	// check if file exists
	if util.IsExist(save_to_file) {
		checksum := crypto.SHA256SumFile(save_to_file)
		if checksum == fileinfo.Checksum {
			logging.Successf("%s already exists, checksum matched", save_to_file)
			return
		} else {
			logging.Warningf("%s already exists, but checksum mismatched", save_to_file)
		}
	}

	err = util.FileAllocate(save_to_file, filesize)
	if err != nil {
		err = fmt.Errorf("GetFile: %s allocate file: %v", file_path, err)
		return
	}
	logging.Printf("We will be downloading %s, %d bytes in total (%s)", file_path, filesize, fileinfo.Checksum)

	// what if we have downloaded part of the file
	var offset int64 = 0
	if util.IsExist(tempname) {
		fiHave := util.FileSize(tempname)
		offset = fiHave
	}

	// mark this file transfer stream
	ftpSh = &network.StreamHandler{}
	// tell agent where to seek the left bytes
	ftpSh.Token = fmt.Sprintf("%s-%s", util.RandMD5String(), fileinfo.Checksum)
	ftpSh.Buf = make(chan []byte)
	ftpSh.BufSize = 1024 * 8
	network.FTPMutex.Lock()
	network.FTPStreams[file_path] = ftpSh
	network.FTPMutex.Unlock()

	// h2x
	ftpSh.H2x = new(def.H2Conn)

	// cmd
	cmd := fmt.Sprintf("get --file_path '%s' --offset %d --token '%s'", file_path, offset, ftpSh.Token)
	err = agents.SendCmd(cmd, "", agent)
	if err != nil {
		logging.Errorf("GetFile send command: %v", err)
		return nil, err
	}

	return ftpSh, nil
}

// DownloadFile download file from URL
func DownloadFile(url string, filepath string) error {
	// Create the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
