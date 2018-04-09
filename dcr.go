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
	"os/user"
)

var (
	app = kingpin.New("dcr", "A repl for docker compose").Author("Rasmus Holm")
	file = app.Flag("file", "Path to docker compose file, if not provided dcr will travers upwards looking for docker-compose.yml").Short('f').String()
	env = app.Flag("env", "Envioriment file for docker compose context, if not provided dcr will try to use .env in the same location as docker-compose").Short('e').String()
	ls = app.Flag("list", "List all avalible docker compose projects").Short('l').Bool()
	repo = app.Arg("compose alias", "The name of the workspace for quick access").String()


	composeObj map[string]interface{}
	groupObj map[string]interface{}
	linereader *readline.Instance
	groupSupport = true;
)

func listServices() func(string) []string {
	return func(line string) []string {
		s := getServices()
		if groupSupport {
			s = getGroups(s)
		}
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
		readline.PcItem("alias"),
		readline.PcItem("reload"),

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


func load(name string, confDir string){
	linereader, _ =readline.NewEx(&readline.Config{
		Prompt:          "\033[32m[" + name +"]>\033[0m ",
		HistoryFile:     confDir + "/" + name + ".history",
		AutoComplete:    completer(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistorySearchFold: true,
	})

}



func main(){
	kingpin.MustParse(app.Parse(os.Args[1:]))


	var composeFile string
	var groupFile string
	var basePath string
	var name string
	var err error

	u, err := user.Current()

	if err != nil {
		log.Fatal(err)
	}

	confDir := u.HomeDir + "/.config/dcr"
	err = exec.Command("mkdir", "-p", confDir).Run()


	if *ls {
		listProjects(confDir)
		return
	}


	if err != nil {
		log.Fatal(confDir, err)
	}

	if *repo != "" {
		name = *repo
		in, err := ioutil.ReadFile(confDir + "/" + name + ".path")
		if err != nil {
			log.Fatal(err)
		}
		in2, err := ioutil.ReadFile(confDir + "/" + name + ".dcrgroups.path")
		if err != nil {
			groupSupport = false;
		} else {
			groupFile = strings.TrimSpace(string(in2))
		}
		composeFile = strings.TrimSpace(string(in))

	}else if *file == ""{
		composeFile = findFile(".", "docker-compose.yml")
		pathParts := strings.Split(composeFile, "/")
		name = pathParts[len(pathParts)-2]
		err = ioutil.WriteFile(confDir + "/" + name + ".path", []byte(composeFile), 0644)
		if err != nil {
			log.Fatal(err)
		}

		groupFile = findFile(".", ".dcrgroups")
		if groupSupport {
			pathParts1 := strings.Split(groupFile, "/")
			name = pathParts1[len(pathParts1)-2]
			err = ioutil.WriteFile(confDir + "/" + name + ".dcrgroups.path", []byte(groupFile), 0644)
			if err != nil {
				fmt.Println("WriteFile Error", err)
			}
		}


	}else{
		composeFile, err = filepath.Abs(*file)
		if err != nil {
			log.Fatal(err)
		}
		pathParts := strings.Split(composeFile, "/")
		name = pathParts[len(pathParts)-2]
		err = ioutil.WriteFile(confDir + "/" + name + ".path", []byte(composeFile), 0644)
		if err != nil {
			log.Fatal(err)
		}


		groupFile, err = filepath.Abs(*file)
		if err != nil {
			groupSupport = false;

		}
		pathParts1 := strings.Split(groupFile, "/")
		name = pathParts1[len(pathParts1)-2]
		err = ioutil.WriteFile(confDir + "/" + name + ".dcrgroups.path", []byte(groupFile), 0644)
		if err != nil {
			groupSupport = false;
		}
	}

	readComposeFile(composeFile)
	err = readGroupFile(groupFile)
	if err != nil {
		fmt.Println("No group support")
		groupSupport = false;
	}

	parts := strings.Split(composeFile, "/")
	basePath = strings.Join(parts[:len(parts)-1], "/")
	if *env == "" {
		a := basePath + "/.env"
		env = &a
	}

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

	load(name, confDir)

	for {

		line, err := linereader.Readline()
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

		switch args[0] {
		case "":
			continue
		case "alias":

			if len(args) != 2{
				fmt.Println("Error, alias need exactly one parameter to be used as the alias for the compose file")
			}

			os.Symlink(confDir+"/" + name + ".history", confDir+"/" + args[1] + ".history")
			os.Symlink(confDir+"/" + name + ".path", confDir+"/" + args[1] + ".path")
			os.Symlink(confDir+"/" + name + ".dcrgroups.path", confDir+"/" + args[1] + ".dcrgroups.path")
			name = args[1]
			fallthrough
		case "reload":
			load(name, confDir)
		case "exit":
			os.Exit(0)
		case "help":

			fmt.Println(`REPL:
Wrapps docker compose and and has a few extra commands

Commands:
  alias              Set alias for current docker compose file
  reload             Reloads docker compose

Docker Compose:`)
			fallthrough
		default:

			envBytes, err := ioutil.ReadFile(*env)
			if err != nil{
				envBytes = []byte("DCR=TRUE")
			}

			if groupSupport {
				for i, arg := range args {
					services := groupObj["groups"]
					for k, l := range services.(map[interface{}]interface{}) {
						if arg == k {
							args[i] = args[len(args)-1]
							args = args[:len(args)-1]
							for _, ss := range l.([]interface{}) {
								args = append(args, ss.(string))
							}
						}
					}
				}
			}

			execArgs := append([]string{string(envBytes), "docker-compose",  "-f", composeFile}, args...)
			cmd := exec.Command("env", execArgs... )
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			err = cmd.Run()
			if err != nil {
				fmt.Println("ERROR", err)
				return
			}
		}

	}
}


func listProjects(confDir string){

	abs ,err := filepath.Abs(confDir)
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

	names := []string{}
	links := []string{}
	maxLen := 0



	fileNames := []string{}
	for _, f := range list {
		fileNames = append(fileNames, f.Name())
	}

	sort.Strings(fileNames)

	for _, name := range fileNames {
		if strings.HasSuffix(name, ".path"){
			cleanName := strings.TrimSuffix(name, ".path")
			link, _ := ioutil.ReadFile(abs + "/" + name)
			names = append(names, cleanName)
			links = append(links, strings.TrimSpace(string(link)))

			if maxLen < len(cleanName){
				maxLen = len(cleanName)
			}

		}

	}

	for i, name := range names{
		fmt.Print(name)
		fmt.Print(strings.Repeat(" ", maxLen- len(name) + 4))
		fmt.Println(links[i])
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


func findFile(dirUri string, fileName string) string{

	abs ,err := filepath.Abs(dirUri)
	if err != nil {
		if fileName == ".dcrgroups" {
			groupSupport = false
			return "";
		}
		log.Fatal(err)
	}

	dir, err := os.Open(abs)

	if err != nil {
		if fileName == ".dcrgroups" {
			groupSupport = false
			return "";
		}
		log.Fatal(err)
	}
	list, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		if fileName == ".dcrgroups" {
			groupSupport = false
			return "";
		}
		log.Fatal(err)
	}


	for _, f := range list {

		if(f.Name() == fileName){
			return abs + "/" + f.Name()
		}
	}

	if abs == "/" {
		if fileName == ".dcrgroups" {
			groupSupport = false
			return "";
		}
		log.Fatal(errors.New("Could not find " + fileName))
	}

	return findFile(abs + "/..", fileName)
}

// Groups

func getGroups (s []string) []string {
	services := groupObj["groups"]

	for k, _ := range services.(map[interface{}]interface{}) {
		s = append(s, k.(string))
	}

	return s
}

func readGroupFile(path string) error{
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	groupObj = make(map[string]interface{})

	err = yaml.Unmarshal(yamlFile, &groupObj)
	if err != nil {
		return err
	}
	return nil
}

