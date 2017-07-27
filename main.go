package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
)

const dockerSock = "unix:///var/run/docker.sock"
const hostsFile = "/etc/hosts"
const customHostsFile = "./myhosts"

type host struct {
	Name string
	IP   string
}

var hosts map[string]host
var customHosts string
var docker *dockerapi.Client

func main() {

	hosts = make(map[string]host)

	var err error
	docker, err = dockerapi.NewClient(dockerSock)
	if err != nil {
		log.Fatal("Error openning:", dockerSock, err)
	}

	events := make(chan *dockerapi.APIEvents)
	err = docker.AddEventListener(events)
	if err != nil {
		log.Fatal("Error subscribing to events:", err)
	}

	copyHostFile()
	customHosts, err = readFile(customHostsFile)
	if err != nil {
		// inform but don't exit
		fmt.Println("Error reading file", customHostsFile, err)
	}

	containers, err := docker.ListContainers(dockerapi.ListContainersOptions{})

	for _, container := range containers {
		fmt.Println("FOUND CONTAINER", container.ID)
		addContainer(container.ID)
	}

	for evt := range events {
		switch evt.Status {
		case "start":
			fmt.Println("FOUND CONTAINER [ADD]", evt.ID)
			addContainer(evt.ID)
			break

		case "die":
			fmt.Println("FOUND CONTAINER [DIE]", evt.ID)
			removeContainer(evt.ID)
			break
		default:
			//fmt.Println("EVENT NOT CAUGHT", evt)
		}
	}

	log.Fatal("Oops! we exited")
}

func addContainer(ID string) {
	container, err := docker.InspectContainer(ID)
	if err != nil {
		fmt.Println("Inspect failed on", ID, err)
		return
	}

	hosts[ID] = host{
		Name: string(container.Name[1:]),
		IP:   container.NetworkSettings.IPAddress,
	}

	fmt.Println("Added container:", hosts[ID])

	// write file
	writeHostFile()
}

func removeContainer(ID string) {
	delete(hosts, ID)

	// write file
	writeHostFile()
}

func readFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stats, statsErr := file.Stat()
	if statsErr != nil {
		return "", statsErr
	}
	var size = stats.Size()
	fw := make([]byte, size)

	bufr := bufio.NewReader(file)
	_, err = bufr.Read(fw)
	return string(fw), err
}

func writeHostFile() {

	content := customHosts + "\n\n\n\n"
	for _, h := range hosts {
		content += h.IP + " " + h.Name + "\n"
	}

	err := ioutil.WriteFile(hostsFile, []byte(content), 0644)
	if err != nil {
		log.Fatal("Error writing host file", hostsFile, content, err)
	}

	fmt.Println(content)

}

func copyHostFile() {

	original, err := readFile(hostsFile)
	if err != nil {
		log.Fatal("Error reading", original, err)
	}

	t := time.Now()
	e := 0
	datestr := fmt.Sprintf("%d%02d%02d", t.Year(), t.Month(), t.Day())
	filename := "hosts." + datestr

	for {
		if !exists(filename) {
			break
		} else {
			e++
			filename = fmt.Sprintf("hosts.%s.%d", datestr, e)
		}
	}

	err = ioutil.WriteFile(filename, []byte(original), 0644)
	if err != nil {
		log.Fatal("Error copying host file", filename, original, err)
	}

}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}
