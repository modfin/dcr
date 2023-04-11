package main

import (
	"fmt"
	"github.com/chzyer/readline"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

var (
	app           = kingpin.New("dcr", "A repl for docker compose").Author("Rasmus Holm")
	file          = app.Flag("file", "Path to docker compose file, if not provided dcr will travers upwards looking for docker-compose.yml").String()
	env           = app.Flag("env", "Environment file for docker compose context, if not provided dcr will try to use .env in the same location as docker-compose.yml").String()
	printComplete = app.Flag("complet-next", "Get command compleation").Hidden().Bool()
	fish          = app.Flag("fish", "Get auto compleation for fish").Bool()
	ls            = app.Flag("list", "List all avalible docker compose projects").Bool()
	repo          = app.Arg("compose alias", "The name of the workspace for quick access").String()
	inargs        = app.Arg("docker-compose command", "The input commant to docker compose").Strings()

	composeObj   map[string]interface{}
	linereader   *readline.Instance
	groupObj     map[string]interface{}
	groupSupport = true
)

type ComposeCompleter struct{}

var composeCompleter *ComposeCompleter = new(ComposeCompleter)

func (ec ComposeCompleter) Do(line []rune, pos int) (suggest [][]rune, retPos int) {
	services := getServices()
	if groupSupport {
		services = getGroups(services)
	}
	comp := map[string][]string{
		"alias":    nil,
		"services": nil,
		"reload":   nil,
		"help":     nil,
		"version":  nil,
		"exit":     nil,
		"build":    services,
		"bundle":   nil,
		"config":   nil,
		"create":   services,
		"down":     nil,
		"events":   services,
		"exec":     services,
		"kill":     services,
		"logs":     services,
		"pause":    services,
		"port":     services,
		"ps":       services,
		"pull":     services,
		"push":     services,
		"restart":  services,
		"rm":       services,
		"run":      services,
		"scale":    services,
		"start":    services,
		"stop":     services,
		"top":      services,
		"unpause":  services,
		"up":       services,
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

			if strings.HasPrefix(alt, part) {
				suggest = append(suggest, []rune(strings.TrimPrefix(alt, part)+" "))
			}

		}
	}

	if len(parts) > 1 {
		alts := comp[parts[0]]
		part := parts[len(parts)-1]
		retPos = len(part)
		if alts == nil {
			return
		}
		for _, alt := range alts {
			if strings.HasPrefix(alt, part) {
				suggest = append(suggest, []rune(strings.TrimPrefix(alt, part)+" "))
			}
		}
	}

	return
}

func load(name string, confDir string) {
	linereader, _ = readline.NewEx(&readline.Config{
		Prompt:            "\033[32m[" + name + "]>\033[0m ",
		HistoryFile:       confDir + "/" + name + ".history",
		AutoComplete:      composeCompleter, //completer(),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
}

func main() {
	rawargs := os.Args[1:]
	cleanedargs := []string{}
	composeCommand := []string{}
	hasRepo := false
	for _, arg := range rawargs {
		switch strings.Split(arg, "=")[0] {
		case "--complet-next":
			fallthrough
		case "--file":
			fallthrough
		case "--env":
			fallthrough
		case "--fish":
			fallthrough
		case "--list":
			fallthrough
		case "--help":
			cleanedargs = append(cleanedargs, arg)
			continue
		}

		if !hasRepo && !strings.HasPrefix(arg, "--") {
			hasRepo = true
			cleanedargs = append(cleanedargs, arg)
		} else {
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

	if err != nil {
		log.Fatal(confDir, err)
	}

	if *ls {
		listProjects(confDir, true)
		return
	}
	if len(os.Args[1:]) == 1 && *printComplete {
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

		in2, err := ioutil.ReadFile(confDir + "/" + name + ".dcrgroups.path")
		if err != nil {
			groupSupport = false
		} else {
			groupFile = strings.TrimSpace(string(in2))
		}

	} else if *file == "" {
		composeFile, err = findFile(".", "docker-compose.yml")
		if err != nil {
			log.Fatalf("could not find compose file, error: %v", err)
		}

		pathParts := strings.Split(composeFile, "/")
		name = pathParts[len(pathParts)-2]
		err = ioutil.WriteFile(confDir+"/"+name+".path", []byte(composeFile), 0644)
		if err != nil {
			log.Fatal(err)
		}

		groupFile, err = findFile(".", ".dcrgroups")

		if err != nil || groupFile == "" {
			groupSupport = false
		}

		if groupSupport {
			pathParts1 := strings.Split(groupFile, "/")
			name = pathParts1[len(pathParts1)-2]
			err = ioutil.WriteFile(confDir+"/"+name+".dcrgroups.path", []byte(groupFile), 0644)
			if err != nil {
				fmt.Println("WriteFile Error", err)
			}
		}

	} else {
		composeFile, err = filepath.Abs(*file)
		if err != nil {
			log.Fatal(err)
		}
		pathParts := strings.Split(composeFile, "/")
		name = pathParts[len(pathParts)-2]
		err = ioutil.WriteFile(confDir+"/"+name+".path", []byte(composeFile), 0644)
		if err != nil {
			log.Fatal(err)
		}

		groupFile, err = filepath.Abs(*file)
		if err != nil {
			groupSupport = false

		}
		pathParts1 := strings.Split(groupFile, "/")
		name = pathParts1[len(pathParts1)-2]
		err = ioutil.WriteFile(confDir+"/"+name+".dcrgroups.path", []byte(groupFile), 0644)
		if err != nil {
			groupSupport = false
		}
	}

	readComposeFile(composeFile)
	err = readGroupFile(groupFile)
	if err != nil {
		//fmt.Println("No group support")
		groupSupport = false
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

	if len(*inargs) > 0 || *printComplete {
		runCommand(*inargs, confDir, name, composeFile)
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

func runCommand(args []string, confDir string, name string, composeFile string) {

	if *printComplete {
		cpl := composeCompleter
		soFar := strings.Join(args, " ")
		if len(soFar) > 1 {
			soFar += " "
		}
		newLine, _ := cpl.Do([]rune(soFar), len(soFar))
		for _, l := range newLine {
			fmt.Println(strings.TrimSpace(string(l)))
		}
		return
	}

	switch args[0] {
	case "":
		return
	case "alias":

		if len(args) != 2 {
			fmt.Println("Error, alias need exactly one parameter to be used as the alias for the compose file")
		}

		os.Symlink(confDir+"/"+name+".history", confDir+"/"+args[1]+".history")
		os.Symlink(confDir+"/"+name+".path", confDir+"/"+args[1]+".path")
		os.Symlink(confDir+"/"+name+".dcrgroups.path", confDir+"/"+args[1]+".dcrgroups.path")
		name = args[1]
		fallthrough
	case "reload":
		load(name, confDir)
	case "exit":
		os.Exit(0)
	case "services":
		arr := getServices()
		for _, s := range arr {
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

		execArgs := []string{"compose"}
		execArgs = append(execArgs, "-f", composeFile)
		execArgs = append(execArgs, composeOverrideArgs(composeFile)...)
		execArgs = append(execArgs, args...)
		cmd := exec.Command("docker", execArgs...)
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

func composeOverrideArgs(composeFile string) []string {
	path := strings.Replace(composeFile, ".yaml", ".override.yaml", 1)
	path = strings.Replace(path, ".yml", ".override.yml", 1)
	_, err := os.Stat(path)
	if err == nil {
		return []string{"-f", path}
	}
	return nil
}

func listProjects(confDir string, full bool) {

	abs, err := filepath.Abs(confDir)
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
		if strings.HasSuffix(name, ".path") && !strings.HasSuffix(name, ".dcrgroups.path") {
			cleanName := strings.TrimSuffix(name, ".path")
			link, _ := ioutil.ReadFile(abs + "/" + name)
			names = append(names, cleanName)
			links = append(links, strings.TrimSpace(string(link)))

			if maxLen < len(cleanName) {
				maxLen = len(cleanName)
			}

		}

	}

	for i, name := range names {
		fmt.Print(name)
		if full {
			fmt.Print(strings.Repeat(" ", maxLen-len(name)+4))
			fmt.Print(links[i])
		}
		fmt.Println()
	}

}

func getServices() []string {

	services := composeObj["services"]
	keys := make([]string, 0, 1)

	for k, _ := range services.(map[interface{}]interface{}) {
		keys = append(keys, k.(string))
	}
	sort.Strings(keys)

	return keys
}

func readComposeFile(path string) {
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

func findFile(dirUri string, fileName string) (string, error) {

	abs, err := filepath.Abs(dirUri)
	if err != nil {
		return "", err
	}

	dir, err := os.Open(abs)

	if err != nil {
		return "", err
	}
	list, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return "", err
	}

	for _, f := range list {

		if f.Name() == fileName {
			return abs + "/" + f.Name(), nil
		}
	}

	if abs == "/" {
		return "", fmt.Errorf("could not find file '%s', last checked dir '%s'", fileName, dirUri)
	}

	return findFile(abs+"/..", fileName)
}

// Groups

func getGroups(s []string) []string {
	services := groupObj["groups"]

	for k, _ := range services.(map[interface{}]interface{}) {
		s = append(s, k.(string))
	}

	return s
}

func readGroupFile(path string) error {
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
