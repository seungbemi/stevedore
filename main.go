package main

import (
	"flag"
	"fmt"
	"gofred"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	noSubtitle     = ""
	noArg          = ""
	noAutocomplete = ""
)

type compose struct {
	Services map[string]interface{}
}

// Config is
type Config struct {
	FilePath    string   `yaml:"filepath"`
	Environment []string `yaml:"environment"`
}

// Message adds simple message
func Message(response *gofred.Response, title, subtitle string, err bool) {
	msg := gofred.NewItem(title, subtitle, noAutocomplete)
	// if err {
	// 	msg = msg.AddIcon(iconError, defaultIconType)
	// } else {
	// 	msg = msg.AddIcon(iconDone, defaultIconType)
	// }
	response.AddItems(msg)
	fmt.Println(response)
}

func init() {
	flag.Parse()
}

func exists(name string) bool {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP
}

const configfile = "config.yml"

func main() {
	path := os.Getenv("PATH")
	if !strings.Contains(path, "/usr/local/bin") {
		os.Setenv("PATH", path+":/usr/local/bin")
	}
	response := gofred.NewResponse()
	content, err := ioutil.ReadFile(configfile)
	if err != nil {
		item := gofred.NewItem("Reading config file error", "Modify Configuration", noAutocomplete).
			AddIcon("plus.png", "").Executable(configfile)
		item.VarMap = make(map[string]string)
		item.VarMap["cmd"] = "filewrite"

		response.AddItems(item)
		fmt.Println(response)
		return
	}

	conf := make(map[string]Config)
	err = yaml.Unmarshal(content, &conf)
	if err != nil {
		item := gofred.NewItem("Parsing config file error", "Modify Configuration", noAutocomplete).
			AddIcon("plus.png", "").Executable(configfile)
		item.VarMap = make(map[string]string)
		item.VarMap["cmd"] = "filewrite"

		response.AddItems(item)
		fmt.Println(response)
		return
	}
	response.VarMap = make(map[string]string)
	configItems := []gofred.Item{}
	selected := ""

	for name, config := range conf {
		if flag.Arg(0) == name {
			selected = name
		} else {
			configItems = append(configItems, gofred.NewItem("Select #"+name, config.FilePath, name).AddIcon("docker.png", ""))
		}
	}
	if len(selected) == 0 {
		response.AddMatchedItems(flag.Arg(0), configItems...)
		item := gofred.NewItem("Modify configuration", noSubtitle, noAutocomplete).
			AddIcon("plus.png", "").Executable(configfile)
		item.VarMap = make(map[string]string)
		item.VarMap["cmd"] = "filewrite"
		response.AddItems(item)
		fmt.Println(response)
		return
	}

	response.VarMap["cmd"] = "bash"
	items := []gofred.Item{}
	comp := compose{}
	if exists(conf[selected].FilePath) {
		content, err := ioutil.ReadFile(conf[selected].FilePath)
		err = yaml.Unmarshal(content, &comp)
		if err != nil {
			Message(response, "Can not parse docker-compose file", "please check if yaml file is well written", true)
			return
		}
	} else {
		item := gofred.NewItem("Can not read docker-compose file", "Set docker-compose file path", noAutocomplete).
			AddIcon("plus.png", "").Executable(configfile)
		item.VarMap = make(map[string]string)
		item.VarMap["cmd"] = "filewrite"

		response.AddItems(item)
		fmt.Println(response)
		return
	}

	var services []string
	for service := range comp.Services {
		services = append(services, service)
	}
	sort.Strings(services)
	outboundIP := getOutboundIP().String()
	envStr := ""
	if len(conf[selected].Environment) > 0 {
		envStr = "env"
		for _, str := range conf[selected].Environment {
			envStr += " " + strings.Replace(str, "localhost", outboundIP, -1)
		}
	}

	baseCommand := fmt.Sprintf("%s /usr/local/bin/docker-compose -f %s ", envStr, conf[selected].FilePath)

	runningServices, err := exec.Command("bash", "-c", fmt.Sprintf("%sps | grep Up | awk '{print $1}'", baseCommand)).CombinedOutput()
	if err != nil {
		if err != nil {
			Message(response, "Can not parse docker-compose file", "please check if yaml file is well written", true)
			return
		}
		return
	}
	if strings.Contains(string(runningServices), "Couldn't connect to Docker daemon") {
		response.AddItems(gofred.NewItem("Docker deamon is not running", "start Docker", noAutocomplete).
			Executable("open -a Docker.app"))
		fmt.Println(response)
		return
	}

	{
		baseItem := gofred.NewItem("All Services", "create & start all services up", noAutocomplete).
			AddIcon("docker.png", "").Executable(baseCommand+"up -d").
			AddOptionKeyAction("Recreate and start all services", baseCommand+"up --force-recreate -d", true).
			AddCommandKeyAction("Stop all services conatainers", baseCommand+"stop", true)
		logItem := gofred.NewItem("Check logs on terminal", "Open terminal and show logs", noAutocomplete).
			AddIcon("docker.png", "").Executable(baseCommand + "logs -f -t")
		logItem.VarMap = make(map[string]string)
		logItem.VarMap["cmd"] = "terminal"

		items = append(items, baseItem, logItem)
	}

	for _, service := range services {
		running := false
		if strings.Contains(string(runningServices), service) {
			running = true
		}

		if running {
			items = append(items, gofred.NewItem(service, "Stop service", noAutocomplete).
				AddIcon("On.png", "").Executable(baseCommand+"stop "+service).
				AddOptionKeyAction("Recreate and start service", baseCommand+"up --force-recreate -d "+service, true))
		} else {
			items = append(items, gofred.NewItem(service, "Start service", noAutocomplete).
				AddIcon("Off.png", "").
				Executable(baseCommand+"up -d "+service).
				AddOptionKeyAction("Recreate and start service", baseCommand+"up --force-recreate -d "+service, true))
		}
	}
	response.AddItems(items...)
	fmt.Println(response)
}
