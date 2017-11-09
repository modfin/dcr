package main

import (
  "gopkg.in/alecthomas/kingpin.v2"
	"github.com/chzyer/readline"
	"io"
	"os"
	"strings"
	"log"
	"path/filepath"
	"github.com/golang-plus/errors"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"sort"
	"os/exec"
	"fmt"
	"syscall"
	"os/signal"
)

var (
	app = kingpin.New("dcr", "A repl for docker compose").Author("Rasmus Holm")
	file = app.Flag("file", "Path to docker compose file, if not provided dcr will travers upwards looking for docker-compose.yml").Short('f').String()
	env = app.Flag("env", "Envioriment file for docker compose context, if not provided dcr will try to use .dcr.env in the same location as docker-compose").Short('e').String()


	composeObj map[string]interface{}
)

func listServices() func(string) []string {
	return func(line string) []string {
		s := getServices()
		sort.Strings(s)
		return s ;
	}
}

func req(i int, f func() func(string) []string, a *readline.PrefixCompleter) *readline.PrefixCompleter {

	if a == nil {
		return req(i-1, f, readline.PcItemDynamic(f()) )
	}

	if i < 1 {
		return a
	}
	return req(i-1, f, readline.PcItemDynamic(f(), a))
}

func completer()(*readline.PrefixCompleter){


	services := req(30, listServices, nil)
	service := req(1, listServices, nil)



	return readline.NewPrefixCompleter(
		readline.PcItem("build", services),
		readline.PcItem("bundle"),
		readline.PcItem("config"),
		readline.PcItem("create", services),
		readline.PcItem("down"),
		readline.PcItem("events", services),
		readline.PcItem("exec", service),
		readline.PcItem("kill", services),
		readline.PcItem("logs", services),
		readline.PcItem("pause", services),
		readline.PcItem("port", service),
		readline.PcItem("ps", services),
		readline.PcItem("pull", services),
		readline.PcItem("push", services),
		readline.PcItem("restart", services),
		readline.PcItem("rm", services),
		readline.PcItem("run", service),
		readline.PcItem("scale", services), // should be// service=num ...
		readline.PcItem("start", services),
		readline.PcItem("stop", services),
		readline.PcItem("top", services),
		readline.PcItem("unpause", services),
		readline.PcItem("up", services),
		readline.PcItem("version"),
		readline.PcItem("help"),
		readline.PcItem("exit"),
	)
}




func main(){
	kingpin.MustParse(app.Parse(os.Args[1:]))


	var composeFile string
	var dir string
	var err error

	if *file == ""{

		composeFile = findFile(".")
	}else{
		composeFile, err = filepath.Abs(*file)
		if err != nil {
			log.Fatal(err)
		}
	}

	readComposeFile(composeFile)

	pathParts := strings.Split(composeFile, "/")
	dir = pathParts[len(pathParts)-2]

	l, _ :=readline.NewEx(&readline.Config{
		Prompt:          "\033[32m[" + dir +"]>\033[0m ",
		HistoryFile:     "/tmp/readline." + dir + ".tmp",
		AutoComplete:    completer(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistorySearchFold: true,
	})

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		for {
			<-sigc
		}
	}()

	for {

		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)

		args := strings.Split(strings.Trim(line, "\n"), " ")


		if args[0] == ""{
			continue
		}else if args[0] == "exit" || args[0] == "quit" {
			os.Exit(0)
		}else{
			cmd := exec.Command("docker-compose", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			err := cmd.Run()
			if err != nil {
				fmt.Println("ERROR", err)
				return
			}
		}

	}
}


func getServices() []string{

	services := composeObj["services"]
	keys := make([]string, 0, 1)

	for k, _ := range services.(map[interface{}]interface{}) {
		keys = append(keys, k.(string))
	}

	return keys
}


func readComposeFile(path string){
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	composeObj = make(map[string]interface{})

	err = yaml.Unmarshal(yamlFile, &composeObj)
	if err != nil {
		log.Fatal(err)
	}

}


func findFile(dirUri string) string{

	abs ,err := filepath.Abs(dirUri)
	if err != nil {
		log.Fatal(err)
	}

	dir, err := os.Open(abs)

	if err != nil {
		log.Fatal(err)
	}
	list, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		log.Fatal(err)
	}


	for _, f := range list {

		if(f.Name() == "docker-compose.yml"){
			return abs + "/" + f.Name()
		}
	}

	if abs == "/" {
		log.Fatal(errors.New("Could not find a docker-compose.yml"))
	}

	return findFile(abs + "/..")
}
