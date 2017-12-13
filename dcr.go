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
	file = app.Flag("file", "Path to docker compose file, if not provided dcr will travers upwards looking for docker-compose.yml").String()
	env = app.Flag("env", "Envioriment file for docker compose context, if not provided dcr will try to use .env in the same location as docker-compose").String()
	printComplete = app.Flag("complet-next", "Get command compleation").Hidden().Bool()
	fish = app.Flag("fish", "Get auto compleation for fish").Bool()
	ls = app.Flag("list", "List all avalible docker compose projects").Bool()
	repo = app.Arg("compose alias", "The name of the workspace for quick access").String()
	inargs = app.Arg("docker-compose command", "The input commant to docker compose").Strings()


	composeObj map[string]interface{}
	linereader *readline.Instance
)





type ComposeCompleter struct{}

type compCmd struct{
	children []string
	flags []string
	noneRecursive bool
}

var composeCompleter *ComposeCompleter = new(ComposeCompleter)

func (ec ComposeCompleter) Do(line []rune, pos int) (suggest [][]rune, retPos int) {
	services := getServices()

	scaleServices := getServices()
	for i := range scaleServices{
		scaleServices[i] = scaleServices[i] + "="
	}

	comp := map[string]compCmd{
		"alias": {},
		"services": {},
		"reload": {},
		"help": {},
		"version": {},
		"exit": {},
		"build": {children:services, 		flags:[]string{"--force-rm", "--no-cache", "--pull"}},
		"bundle": {											flags:[]string{"--push-images", "--output"}},
		"config": {											flags:[]string{"--quiet", "--services"}},
		"create": {children:services, 	flags:[]string{"--force-recreate", "--no-recreate", "--no-build", "--build"}},
		"down": {												flags:[]string{"--rmi", "--volumes", "--remove-orphans"}},
		"events": {children:services, 	flags:[]string{"--json"}},
		"exec": {children:services, 	  flags:[]string{"-d", "--privileged", "--user", "-T", "--index="}, noneRecursive:true,},
		"kill": {children:services, 		flags:[]string{"-s"}},
		"logs": {children:services, 		flags:[]string{"--no-color", "--follow", "--timestamps", "--tail="}},
		"pause": {children:services},
		"port": {children:services, 		flags:[]string{"--protocol=", "--index="}},
		"ps": {children:services, 			flags:[]string{"-q"}},
		"pull": {children:services, 		flags:[]string{"--ignore-pull-failures"}},
		"push": {children:services, 		flags:[]string{"--ignore-push-failures"}},
		"restart": {children:services, 	flags:[]string{"--timeout"}},
		"rm": {children:services, 			flags:[]string{"--timeout", "-v", "--all"}},
		"run": {children:services, 			flags:[]string{
			"-d", "--name", "--entrypoint", "-e", "--user=",
			"--no-deps", "--rm", "--publish=", "--service-ports",
			"-T", "--workdir=",
		}},
		"scale": {children:scaleServices, 		flags:[]string{"--timeout"}},
		"start": {children:services},
		"stop": {children:services, 		flags:[]string{"--timeout"}},
		"top": {children:services},
		"unpause": {children:services, 	flags:[]string{"--rmi"}},
		"up": {children:services, 			flags:[]string{
			"-d", "--no-color", "--no-deps", "--force-recreate",
			"--no-recreate", "--no-build", "--build", "--abort-on-container-exit",
			"--timeout", "--remove-orphans",
		}},
	}




	str := string(line)
	parts := strings.Split(str, " ")
	suggest = [][]rune{}

	if len(parts) == 0 {
		parts = []string{""}
	}

	if len(parts) == 1 {

		part := parts[0]
		retPos = len(part)
		for alt := range comp {

			if strings.HasPrefix(alt, part){
				suggest = append(suggest, []rune(strings.TrimPrefix(alt, part) + " ") )
			}

		}
	}

	if len(parts) > 1 {

		compCmd := comp[parts[0]]
		part := parts[len(parts)-1]
		retPos = len(part)
		if compCmd.children == nil {
			return
		}

		if len(parts) == 2 || strings.HasPrefix(parts[len(parts)-2], "-") {
			for _, flag := range compCmd.flags {

				if strings.HasPrefix(flag, part) {
					suffix := " "
					if strings.HasSuffix(flag, "=") {
						suffix = ""
					}
					suggest = append(suggest, []rune(strings.TrimPrefix(flag, part) + suffix))
				}
			}

		}


		for _, alt := range compCmd.children{
			if strings.HasPrefix(alt, part){
				suffix := " "
				if strings.HasSuffix(alt, "="){
					suffix = ""
				}
				suggest = append(suggest, []rune(strings.TrimPrefix(alt, part) + suffix) )
			}
		}



	}

	return
}



func load(name string, confDir string){
	linereader, _ =readline.NewEx(&readline.Config{
		Prompt:          "\033[32m[" + name +"]>\033[0m ",
		HistoryFile:     confDir + "/" + name + ".history",
		AutoComplete:    composeCompleter,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistorySearchFold: true,
	})

}



func main(){


	rawargs := os.Args[1:]
	cleanedargs := []string{}
	composeCommand := []string{}
	hasRepo := false
	for _, arg := range rawargs{
		switch strings.Split(arg, "=")[0] {
		case "--complet-next": fallthrough
		case "--file": fallthrough
		case "--env": fallthrough
		case "--fish": fallthrough
		case "--list": fallthrough
		case "--help":
			cleanedargs = append(cleanedargs, arg)
			continue
		}

		if !hasRepo && !strings.HasPrefix(arg, "--"){
			hasRepo = true
			cleanedargs = append(cleanedargs, arg)

		}else{
			composeCommand = append(composeCommand, arg)
		}

	}
	kingpin.MustParse(app.Parse(cleanedargs))
	inargs = &composeCommand


	if *fish {
		fmt.Println(`#Put this in ~/.config/fish/completions or /usr/share/fish/vendor_completions.d
function __fish_get_dcr_command
  set cmd (commandline -opc)
  eval $cmd --complet-next
end
complete -f -c dcr -a "(__fish_get_dcr_command)"`)
		return
	}



	var composeFile string
	var basePath string
	var name string
	var err error

	u, err := user.Current()

	if err != nil {
		log.Fatal(err)
	}

	confDir := u.HomeDir + "/.config/dcr"
	err = exec.Command("mkdir", "-p", confDir).Run()

	if err != nil {
		log.Fatal(confDir, err)
	}

	if *ls {
		listProjects(confDir, true)
		return
	}
	if (len(os.Args[1:]) == 1  && *printComplete ){
		fmt.Println(".")
		listProjects(confDir, false)
		return
	}


	if *repo != "" && *repo != "." {
		name = *repo
		in, err := ioutil.ReadFile(confDir + "/" + name + ".path")
		if err != nil {

			log.Fatal(err)
		}
		composeFile = strings.TrimSpace(string(in))

	}else if *file == ""{
		composeFile = findFile(".")
		pathParts := strings.Split(composeFile, "/")
		name = pathParts[len(pathParts)-2]
		err = ioutil.WriteFile(confDir + "/" + name + ".path", []byte(composeFile), 0644)
		if err != nil {
			log.Fatal(err)
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
	}

	readComposeFile(composeFile)

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

	if len(*inargs) > 0 || *printComplete {
		runCommand(*inargs, confDir,name,composeFile)
		return
	}

	// Run REPL
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

		runCommand(args, confDir, name, composeFile)

	}
}


func runCommand(args []string, confDir string, name string, composeFile string){

	if *printComplete {
		cpl := composeCompleter
		soFar := strings.Join(args, " ")
		if len(soFar) > 1{
			soFar += " "
		}
		newLine, _ := cpl.Do([]rune(soFar), len(soFar))
		for _, l := range newLine{
			fmt.Println(strings.TrimSpace(string(l)))
		}
		return
	}

	switch args[0] {
	case "":
		return
	case "alias":

		if len(args) != 2{
			fmt.Println("Error, alias need exactly one parameter to be used as the alias for the compose file")
		}

		os.Symlink(confDir+"/" + name + ".history", confDir+"/" + args[1] + ".history")
		os.Symlink(confDir+"/" + name + ".path", confDir+"/" + args[1] + ".path")
		name = args[1]
		fallthrough
	case "reload":
		load(name, confDir)
	case "exit":
		os.Exit(0)
	case "services":
		arr := getServices()
		for _, s := range arr{
			fmt.Println(s)
		}


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

func listProjects(confDir string, full bool){

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
		if full {
			fmt.Print(strings.Repeat(" ", maxLen - len(name) + 4))
			fmt.Print(links[i])
		}
		fmt.Println()
	}

}

func getServices() []string{

	services := composeObj["services"]
	keys := make([]string, 0, 1)

	for k, _ := range services.(map[interface{}]interface{}) {
		keys = append(keys, k.(string))
	}
	sort.Strings(keys)

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
