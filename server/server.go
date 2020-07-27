package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
)

// server - take in url
// upload path
// browse path
// path for download

type Server struct {
	instance    *http.Server
	hostAddress string
	wait        chan bool
}

type Config struct {
	Port string
}

func (s *Server) Wait() {
	<-s.wait
}
func (s *Server) Welcome() {
	fmt.Printf("Listening and running on %s\n", s.hostAddress)
	fmt.Println("Enter the address above to list, send or download")
}

func NewServer(config Config) (*Server, error) {
	httpServer := &http.Server{}
	server := &Server{}
	server.instance = httpServer
	server.wait = make(chan bool)
	var port string
	if config.Port == " " {
		port = "8080"
	} else {
		port = config.Port
	}
	server.hostAddress = getHostAddress(port)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Welcome you have made it so far")
	})
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
