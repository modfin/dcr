package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/modfin/dcr/internal/compose"
	"github.com/modfin/dcr/internal/configs"
	"github.com/modfin/dcr/internal/groups"
	"github.com/modfin/dcr/internal/integrations/zdap"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

var (
	app           = kingpin.New("dcr", "A repl for docker compose").Author("Rasmus Holm")
	file          = app.Flag("file", "Path to docker compose file, if not provided dcr will travers upwards looking for docker-compose.yml").String()
	config        = app.Flag("config", "Path to dcr config file, if not provided dcr will look for .dcrconfig in the same location as docker-compose.yml").String()
	env           = app.Flag("env", "Environment file for docker compose context, if not provided dcr will try to use .env in the same location as docker-compose.yml").String()
	printComplete = app.Flag("complet-next", "Get command compleation").Hidden().Bool()
	fish          = app.Flag("fish", "Get auto compleation for fish").Bool()
	ls            = app.Flag("list", "List all avalible docker compose projects").Bool()
	repo          = app.Arg("compose alias", "The name of the workspace for quick access").String()
	zdapFlag      = app.Flag("zdap", "Run zdap integration").Default("false").Bool()
	inargs        = app.Arg("docker-compose command", "The input commant to docker compose").Strings()

	composeObj   compose.Compose
	linereader   *readline.Instance
	groupObj     *groups.Groups
	groupSupport = true
)

type ComposeCompleter struct{}

var composeCompleter *ComposeCompleter = new(ComposeCompleter)

func (ec ComposeCompleter) Do(line []rune, pos int) (suggest [][]rune, retPos int) {
	services := compose.GetServices(composeObj)
	if groupSupport {
		services = groups.GetGroups(services, groupObj)
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
		case "--zdap":
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
	var configFile string
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

	if *config != "" {
		configFile, err = filepath.Abs(*config)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		configFile, err = findFile(filepath.Dir(composeFile), ".dcrconfig.yaml")
		if err != nil {
			configFile = confDir + "/.dcrconfig.yaml"
			err = os.WriteFile(configFile, []byte(""), 0644)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	readComposeFile(composeFile)
	groupObj, err = readGroupFile(groupFile)
	if err != nil {
		//fmt.Println("No group support")
		groupSupport = false
	}
	argsCopy := make([]string, len(*inargs))
	copy(argsCopy, *inargs)
	err = runConfigs(configFile, argsCopy, composeObj, groupObj)
	if err != nil {
		fmt.Println("Error when executing config:", err.Error())
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
		os.Exit(runCommand(*inargs, confDir, name, composeFile))
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

func runCommand(args []string, confDir string, name string, composeFile string) int {

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
		return 0
	}

	switch args[0] {
	case "":
		return 0
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
		arr := compose.GetServices(composeObj)
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
			args = groups.GetGroupServices(args, groupObj)
		}

		execArgs := []string{"compose"}
		execArgs = append(execArgs, "--project-name", sanitizeProjectName(name))
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
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return 1
		}
	}
	return 0
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

func readComposeFile(path string) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	composeObj = compose.Compose{
		Services: make(map[string]compose.Service),
	}

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

// Configs
func runConfigs(path string, args []string, composeObj compose.Compose, groupObj *groups.Groups) error {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config configs.Config
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		return err
	}

	isPreStart := configs.IsPreStart(args)

	// --- Process Pre-Start Actions ---
	preStartActions := config.ActionConfig.PreStartActions
	if isPreStart && len(preStartActions.Actions) > 0 && preStartActions.Enabled {
		services := compose.GetFilteredServices(compose.GetServices(composeObj), preStartActions.Filter, []string{})
		fmt.Println("Running pre-start actions...")
		for _, service := range services {
			err := configs.RunActions(preStartActions.Actions, config.ActionConfig.ActionTemplates, service, preStartActions.Verbose)
			if err != nil {
				return fmt.Errorf("Error running pre-start actions: %v", err)
			}
		}
	}

	// --- Process Integrations ---
	zdapConfig := config.Integrations.Zdap
	if isPreStart && len(zdapConfig.PreStartActions.Actions) > 0 && *zdapFlag || zdapConfig.PreStartActions.Enabled {
		err := zdap.RunPreStartActions(&config, args, composeObj, groupObj)
		if err != nil {
			return fmt.Errorf("error running zdap integration: %v", err)
		}
	}

	return nil
}

func readGroupFile(path string) (*groups.Groups, error) {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	groupObj := groups.Groups{
		Groups: make(map[string][]string),
	}

	err = yaml.Unmarshal(yamlFile, &groupObj)
	if err != nil {
		return nil, err
	}
	return &groupObj, nil
}

// Project names must contain only lowercase letters, decimal digits, dashes, and underscores, and
// must begin with a lowercase letter or decimal digit. If the basename of the project directory or
// current directory violates this constraint, you must use one of the other mechanisms.
// from: https://docs.docker.com/compose/reference/#use--p-to-specify-a-project-name
func sanitizeProjectName(name string) string {
	allowedChars := regexp.MustCompile(`[^a-z0-9_-]+$`)
	allowedStartChar := regexp.MustCompile(`^[a-z]`)
	sanitizedName := allowedChars.ReplaceAllString(strings.ToLower(name), "")
	if len(sanitizedName) > 0 && !allowedStartChar.MatchString(sanitizedName[:1]) {
		sanitizedName = "project-" + sanitizedName
	}
	return sanitizedName
}
