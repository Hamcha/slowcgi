package main

import (
	"flag"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type cgiHandler struct {
	cgiDir string
}

func (c *cgiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := c.cgiDir + r.URL.Path
	/* Check for file errors */
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.Error(w, "Not found", 404)
		return
	}
	if os.IsPermission(err) {
		http.Error(w, "Forbidden", 403)
		return
	}

	cmd := exec.Command(path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, "Internal server error", 500)
	}

	w.Write([]byte(out))
}

func main() {
	/* Get parameters */
	var socketFile = flag.String("s", "/var/run/slowcgi.sock", "Socket file")
	var serveDir = flag.String("d", "/var/www/cgi-bin", "Directory to serve files from")
	var listenHTTP = flag.Bool("http", false, "Listen as a standalone HTTP server instead of FastCGI")
	flag.Parse()

	socket, err := net.Listen("unix", *socketFile)
	if err != nil {
		panic(err.Error())
	}

	/* Chmod so everyone can write or Nginx will cry */
	os.Chmod(*socketFile, 0777)

	/* Setup SIGINT/SIGTERM listener so we can clean our mess */
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		socket.Close()
		os.Remove(*socketFile)
		os.Exit(1)
	}()

	/* Setup handler and start listening */
	cgi := &cgiHandler{cgiDir: *serveDir}
	if *listenHTTP {
		http.Serve(socket, cgi)
	} else {
		fcgi.Serve(socket, cgi)
	}
}
