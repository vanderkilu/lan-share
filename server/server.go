package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// server - take in url
// upload path
// browse path
// path for download

type Server struct {
	instance    *http.Server
	hostAddress string
	errorChan   chan bool
	browsePath  string
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
	http.HandleFunc("/browse", func(w http.ResponseWriter, r *http.Request) {
		var files []BrowseFile
		if s.browsePath != "" {
			err := filepath.Walk(s.browsePath, func(path string, info os.FileInfo, err error) error {
				browseFile := BrowseFile{}
				fullPath := filepath.Join(s.browsePath, path)
				//Todo, check if directory file is already included
				if info.IsDir() {
					browseFile.IsDir = true
					browseFile.Path = s.browsePath
				} else {
					browseFile.IsDir = false
					browseFile.Path = fullPath
				}

				files = append(files, browseFile)
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
}

func (s *Server) SetMountPath(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Println("Path provided could not be resolved")
		s.errorChan <- true
	}
	s.browsePath = absPath

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
