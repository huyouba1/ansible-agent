package ansible

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
)

type Server struct {
	m *martini.Martini
}

var (
	AuthToken string
)

func NewServer() *Server {
	s := &Server{}
	r := martini.NewRouter()
	r.Get("/ping", s.Ping)
	r.Post("/exec", s.ExecCommand)
	r.Put("/upload", s.PutFile)

	m := martini.New()
	m.Use(martini.Logger())
	m.Use(martini.Recovery())
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	s.m = m
	return s
}

func (s *Server) ConfigureLDAP(options *LdapOptions) error {
	handler, err := LdapAuthenticator(options)
	if err != nil {
		return err
	}
	s.m.Use(handler)
	return nil
}

func (s *Server) ValidateHeader(req *http.Request) error {
	AccessToken := ""
	auth := req.Header.Get("Authorization")
	al := strings.Split(auth, " ")
	if len(al) > 1 {
		AccessToken = al[1]
	} else {
		AccessToken = auth
	}
	if AccessToken != AuthToken {
		return fmt.Errorf("认证失败，token不合法")
	}
	return nil
}

func (s *Server) Serve(l net.Listener) error {
	return http.Serve(l, s.m)
}

func (s *Server) Ping() []byte {
	serverInfo := map[string]string{}
	out, _ := json.Marshal(&serverInfo)
	return out
}

func (s *Server) ExecCommand(req *http.Request) (int, interface{}) {
	if err := s.ValidateHeader(req); err != nil {
		return http.StatusUnauthorized, "认证失败，token不合法\n"
	}
	command := req.FormValue("command")
	if command == "" {
		return http.StatusInternalServerError, "command is a required parameter\n"
	}

	executable := req.FormValue("executable")
	if executable == "" {
		executable = "/bin/sh"
	}

	become := false
	if arg := req.FormValue("become"); arg != "" {
		value, err := strconv.Atoi(arg)
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("error decoding 'become' value: %s", err)
		}

		if value != 0 {
			become = true
		}
	}

	becomeMethod := req.FormValue("becomeMethod")
	if becomeMethod == "" {
		becomeMethod = "sudo"
	}

	// if the /exec request contains stdin, we are likely pipelining
	// if some other error happens, we want to report the error and exit
	// read all of stdin and write to a temporary file, then pipe that file to the process
	// interpreters have a bad habit of executing files before they have finished transferring
	// and can be a vector for security vulnerabilities when a file is only half-read.
	var stdin io.ReadCloser
	if strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		input, _, err := req.FormFile("stdin")
		if err != nil && err != http.ErrMissingFile {
			return http.StatusInternalServerError, fmt.Sprintf("%s\n", err.Error())
		}

		tmpfile, err := ioutil.TempFile(os.TempDir(), "ansible-stdin")
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("%s\n", err.Error())
		}
		defer os.Remove(tmpfile.Name())

		_, err = io.Copy(tmpfile, input)
		tmpfile.Close()
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("%s\n", err.Error())
		}

		stdin, err = os.Open(tmpfile.Name())
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("%s\n", err.Error())
		}
		defer stdin.Close()
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	// preallocate the command array (we have a maximum of 5 elements at the moment)
	cmdArgs := make([]string, 0, 5)
	if become {
		switch becomeMethod {
		case "sudo":
			cmdArgs = append(cmdArgs, "sudo", "-n")
		default:
			return http.StatusInternalServerError, fmt.Sprintf("unsupported become method '%s'", becomeMethod)
		}
	}
	cmdArgs = append(cmdArgs, executable, "-c", command)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()

	data := map[string]interface{}{}
	if err != nil {
		data["status"] = 1
	} else {
		data["status"] = 0
	}
	data["stdin"] = ""
	data["stdout"] = stdout.String()
	data["stderr"] = stderr.String()

	out, err := json.Marshal(&data)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	return http.StatusOK, out
}

func (s *Server) PutFile(req *http.Request) (int, string) {
	if err := s.ValidateHeader(req); err != nil {
		return http.StatusUnauthorized, "认证失败，token不合法\n"
	}
	dest := req.FormValue("dest")
	src, _, err := req.FormFile("src")
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	f, err := os.Create(dest)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	defer f.Close()

	io.Copy(f, src)
	return http.StatusOK, ""
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.m.ServeHTTP(res, req)
}
