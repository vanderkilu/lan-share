package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type Server struct {
	instance     *http.Server
	hostAddress  string
	errorChan    chan bool
	browsePath   string
	downloadPath string
}

type Config struct {
	Port string
}

type BrowseFile struct {
	Path  string `json: "path"`
	IsDir bool   `json: "isDir"`
}

func (s *Server) Wait() {
	<-s.errorChan
}
func (s *Server) Welcome() {
	fmt.Printf("Listening and running on %s\n", s.hostAddress)
	fmt.Println("Enter the address above to list, send or download")
}

func (s *Server) handleRequests() {

	//mount a file path and browse files in it
	http.HandleFunc("/browse", func(w http.ResponseWriter, r *http.Request) {
		var files []BrowseFile
		if s.browsePath != "" {
			err := filepath.Walk(s.browsePath, func(path string, info os.FileInfo, err error) error {
				browseFile := BrowseFile{}
				fullPath := filepath.Join(s.browsePath, path)

				if info.IsDir() {
					browseFile.IsDir = true
					browseFile.Path = s.browsePath
				} else {
					browseFile.IsDir = false
					browseFile.Path = fullPath
				}

				//account for duplicate file/dir names
				_, isIn := find(files, browseFile.Path)

				if !isIn {
					files = append(files, browseFile)
				}
				return nil
			})
			if err != nil {
				fmt.Println("An error occurred reading mount path contents")
				s.errorChan <- true
			}
			json.NewEncoder(w).Encode(files)
		} else {
			fmt.Println("No directory or browse path provided")
			s.errorChan <- true
		}
	})

	//Download a file by name
	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		if s.downloadPath == "" {
			fmt.Println("No download path specified")
			s.errorChan <- true
		}
		path := s.downloadPath
		//check if dir, compress and create tar file

		if isDir(path) {
			err := compressDir(path)
			if err != nil {
				fmt.Println("Unable to zip folder for download")
				s.errorChan <- true
			} else {
				path = fmt.Sprintf("%s.tar.gzip", filepath.ToSlash(path))
				fmt.Println(path)
			}
		}

		fileName := filepath.Base(path)

		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileName))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	})
}

func (s *Server) SetPath(path string, isMountPath bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Println("Path provided could not be resolved")
		s.errorChan <- true
	}
	if isMountPath {
		s.browsePath = absPath
	} else {
		s.downloadPath = absPath
	}

}

func NewServer(config Config) (*Server, error) {
	httpServer := &http.Server{}
	server := &Server{}
	server.instance = httpServer
	server.errorChan = make(chan bool)
	var port string
	if config.Port == " " {
		port = "8080"
	} else {
		port = config.Port
	}
	server.hostAddress = getHostAddress(port)

	//call handlers
	server.handleRequests()

	go func() {
		listener, err := net.Listen("tcp", server.hostAddress)
		if err != nil {
			fmt.Println("Couldn't create the server")
			return
		}
		if err := httpServer.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()
	return server, nil
}

func find(slice []BrowseFile, path string) (int, bool) {
	for i, file := range slice {
		if file.Path == path {
			return i, true
		}
	}
	return -1, false
}

//adapted from https://gist.github.com/mimoo/25fc9716e0f1353791f5908f94d6e726
func compressDir(dir string) error {
	var buff bytes.Buffer
	zipW := gzip.NewWriter(&buff)
	tarW := tar.NewWriter(zipW)

	filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		// generate header(metadata)
		header, err := tar.FileInfoHeader(info, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(file)
		//write the header
		if err := tarW.WriteHeader(header); err != nil {
			return err
		}

		//check if not a dir write content
		if !info.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			//write file content to tar
			if _, err := io.Copy(tarW, data); err != nil {
				return err
			}
		}
		return nil
	})
	//actually create the tar file
	if err := tarW.Close(); err != nil {
		return err
	}
	//create gzip
	if err := zipW.Close(); err != nil {
		return err
	}
	return nil

}

func getHostAddress(port string) string {
	localIP := getOutboundIP()
	return fmt.Sprintf("%s:%s", localIP, port)
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func isDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := fileInfo.Mode()
	if mode.IsDir() {
		return true
	}
	return false
}
