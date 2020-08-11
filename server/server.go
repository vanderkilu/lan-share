package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"../utils"
)

type Server struct {
	instance     *http.Server
	hostAddress  string
	errorChan    chan bool
	browsePath   string
	downloadPath string
	uploadPath   string
}

type Config struct {
	Port string
}

type BrowseFile struct {
	Path     string `json: "path"`
	IsDir    bool   `json: "isDir"`
	BaseName string `json: "baseName"`
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
					browseFile.Path = fullPath
					browseFile.BaseName = filepath.Base(fullPath)
				} else {
					browseFile.IsDir = false
					browseFile.Path = fullPath
					browseFile.BaseName = filepath.Base(fullPath)
				}

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
		contentType := "application/octet-stream"
		//check if dir, compress and create tar file

		if isDir(path) {
			fmt.Println("zipping up folder")
			path = fmt.Sprintf("%s.zip", filepath.ToSlash(path))
			contentType = "application/zip"

			err := utils.CompressDir(s.downloadPath, path)
			if err != nil {
				fmt.Println("error creating zip folder")
				s.errorChan <- true
			}
		}

		fileName := filepath.Base(path)
		fmt.Println(fileName)

		defer os.Remove(path)

		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileName))
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, path)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		absTempPath, _ := filepath.Abs("./views")
		// create file reader from request
		if r.Method == "GET" {
			tmpl := template.Must(template.ParseFiles(absTempPath + "/upload.html"))
			type Data struct {
				Route string
			}
			data := Data{Route: "/upload"}
			tmpl.Execute(w, data)
		} else {
			reader, err := r.MultipartReader()
			if err != nil {
				fmt.Println("Error creating file reader")
				s.errorChan <- true
			}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if part.FileName() == "" {
					continue
				}
				fileName := part.FileName()

				var uploadDir string

				if s.uploadPath == "" {
					absUploadPath, _ := filepath.Abs("./tmp")
					uploadDir = absUploadPath
				}
				uploadFile, err := os.Create(filepath.Join(uploadDir, fileName))
				if err != nil {
					fmt.Println("There was an error creating upload directory")
					s.errorChan <- true
				}
				defer uploadFile.Close()
				buff := make([]byte, 1024)
				for {
					chunk, err := part.Read(buff)
					if err != nil && err != io.EOF {
						fmt.Println(err)
						fmt.Println("Error writing file to disk")
						s.errorChan <- true
						return
					}
					if chunk == 0 {
						break
					}
					if _, err := uploadFile.Write(buff[:chunk]); err != nil {
						fmt.Println("Error writing file to disk")
						s.errorChan <- true
						return
					}
				}

			}
			tmpl := template.Must(template.ParseFiles("../views/uploadDone.html"))
			tmpl.Execute(w, nil)
		}

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

//todo
// structure upload well
// make upload path dynamic
// create a function to handle upload
// allow multiple uploads
// allow folder uploads
