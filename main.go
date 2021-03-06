package recordplugin

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/Monibuca/engine"
	. "github.com/Monibuca/engine/util"
)

var config = struct {
	Path        string
	AutoPublish bool
}{}
var recordings = sync.Map{}

type FlvFileInfo struct {
	Path     string
	Size     int64
	Duration uint32
}

func init() {
	InstallPlugin(&PluginConfig{
		Name:    "Record",
		Type:    PLUGIN_SUBSCRIBER,
		Config:  &config,
		Version: "1.0.0",
		UI:      CurrentDir("dashboard", "ui", "plugin-record.min.js"),
		Run:     run,
	})
}
func run() {
	OnSubscribeHooks.AddHook(onSubscribe)
	os.MkdirAll(config.Path, 0666)
	http.HandleFunc("/record/flv/list", func(writer http.ResponseWriter, r *http.Request) {
		if files, err := tree(config.Path, 0); err == nil {
			var bytes []byte
			if bytes, err = json.Marshal(files); err == nil {
				writer.Write(bytes)
			} else {
				writer.Write([]byte("{\"err\":\"" + err.Error() + "\"}"))
			}
		} else {
			writer.Write([]byte("{\"err\":\"" + err.Error() + "\"}"))
		}
	})
	http.HandleFunc("/record/flv", func(writer http.ResponseWriter, r *http.Request) {
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			if err := SaveFlv(streamPath, r.URL.Query().Get("append") != ""); err != nil {
				writer.Write([]byte(err.Error()))
			} else {
				writer.Write([]byte("success"))
			}
		} else {
			writer.Write([]byte("no streamPath"))
		}
	})

	http.HandleFunc("/record/flv/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			filePath := filepath.Join(config.Path, streamPath+".flv")
			if stream, ok := recordings.Load(filePath); ok {
				output := stream.(*OutputStream)
				output.Close()
				w.Write([]byte("success"))
			} else {
				w.Write([]byte("no query stream"))
			}
		} else {
			w.Write([]byte("no such stream"))
		}
	})
	http.HandleFunc("/record/flv/play", func(w http.ResponseWriter, r *http.Request) {
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			if err := PublishFlvFile(streamPath); err != nil {
				w.Write([]byte(err.Error()))
			} else {
				w.Write([]byte("success"))
			}
		} else {
			w.Write([]byte("no streamPath"))
		}
	})
	http.HandleFunc("/record/flv/delete", func(w http.ResponseWriter, r *http.Request) {
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			filePath := filepath.Join(config.Path, streamPath+".flv")
			if Exist(filePath) {
				if err := os.Remove(filePath); err != nil {
					w.Write([]byte(err.Error()))
				} else {
					w.Write([]byte("success"))
				}
			} else {
				w.Write([]byte("no such file"))
			}
		} else {
			w.Write([]byte("no streamPath"))
		}
	})
}
func onSubscribe(s *OutputStream) {
	filePath := filepath.Join(config.Path, s.StreamPath+".flv")
	if s.Publisher == nil && Exist(filePath) {
		go PublishFlvFile(s.StreamPath)
	}
}

func tree(dstPath string, level int) (files []*FlvFileInfo, err error) {
	var dstF *os.File
	dstF, err = os.Open(dstPath)
	if err != nil {
		return
	}
	defer dstF.Close()
	fileInfo, err := dstF.Stat()
	if err != nil {
		return
	}
	if !fileInfo.IsDir() { //如果dstF是文件
		if path.Ext(fileInfo.Name()) == ".flv" {
			files = append(files, &FlvFileInfo{
				Path:     strings.TrimPrefix(strings.TrimPrefix(dstPath, config.Path), "/"),
				Size:     fileInfo.Size(),
				Duration: getDuration(dstF),
			})
		}
		return
	} else { //如果dstF是文件夹
		var dir []os.FileInfo
		dir, err = dstF.Readdir(0) //获取文件夹下各个文件或文件夹的fileInfo
		if err != nil {
			return
		}
		for _, fileInfo = range dir {
			var _files []*FlvFileInfo
			_files, err = tree(filepath.Join(dstPath, fileInfo.Name()), level+1)
			if err != nil {
				return
			}
			files = append(files, _files...)
		}
		return
	}

}
