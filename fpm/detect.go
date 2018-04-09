package fpm

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/keybase/go-ps"
	psutil "github.com/shirou/gopsutil/process"
	"github.com/tomasen/fcgi_client"
	"gopkg.in/square/go-jose.v2/json"
)

// Structure describing the detected features
type PhpFpmConfig struct {
	// Detected listen URL
	// Can be either localhost:port or full path
	// to a local socket file
	ListenAddress string
	// Type of network connection (unix|tcp)
	ListenNetwork string

	// Version of PHP
	PhpVersion *version.Version
	// List of PHP loaded extensions
	PhpExtensions []string

	// Unix user who will need write access for php
	PhpUser string
	// Unix group who will need write access for php
	PhpGroup string
	// Unix owner of the socket
	ListenOwner string
	// Unix group of the socket
	ListenGroup string
}

// DetectFpmInfos first tries to connect to common addresses for FPM daemon,
// then tries to find a running process and parse its config by sending php-fpm -tt command.
func DetectFpmInfos() (config *PhpFpmConfig, e error) {

	config = &PhpFpmConfig{}

	// Tries to directly connect to common addresses
	detectByDirectConnection(config)

	if config.ListenAddress == "" {

		if proc, e := findRunningBinary(); e == nil {
			// Could find the process running on this machine, try to run a 'php-fpm -tt' command
			if _, e := parseCommandConfig(proc, config); e == nil {
				if e := detectByDirectConnection(config); e != nil {
					return config, e
				}
			}
		}

		return nil, fmt.Errorf("cannot find any suitable configuration for php-fpm")
	} else {
		log.Println(config)
	}

	return
}

// DetectPhpInfos sends a couple of PHP scripts to a running FPM daemon.
func DetectPhpInfos(configs *PhpFpmConfig, scriptsFolder string) error {

	prepareFiles(scriptsFolder)

	versionScript := filepath.Join(scriptsFolder, "version.php")
	log.Println(versionScript)
	output, e := phpGetAsBytes(versionScript, configs)
	if e != nil {
		return e
	}
	if v, e := version.NewVersion(strings.Trim(string(output), " ")); e == nil {
		configs.PhpVersion = v
	}

	extensionsScript := filepath.Join(scriptsFolder, "extensions.php")
	output, e = phpGetAsBytes(extensionsScript, configs)
	if e != nil {
		return e
	}
	var extensions []string
	if e := json.Unmarshal(output, &extensions); e != nil {
		return e
	}
	configs.PhpExtensions = extensions

	cleanFiles(scriptsFolder)

	return nil
}

// detectByDirectConnection tries to Dial a connection to php. If the config is set, it uses the ListenNetwork and
// ListenAddress parameters to test the connection, and returns on error. Otherwise, it tries with most common values
// (tcp port 9000 or unix sockets inside /run/php) and feeds the config if one matches.
func detectByDirectConnection(config *PhpFpmConfig) error {

	if config.ListenAddress != "" {
		client, err := fcgiclient.DialTimeout(config.ListenNetwork, config.ListenAddress, 100*time.Millisecond)
		if err == nil {
			log.Println("Successfully connected to ", config.ListenNetwork, config.ListenAddress)
			client.Close()
			return nil
		} else {
			return err
		}
	}

	commons := map[string][]string{
		"unix": {
			"/run/php/php-fpm.sock",
			"/run/php/php7.0-fpm.sock",
			"/run/php/php7.1-fpm.sock",
			"/run/php/php7.2-fpm.sock",
			"/run/php/php70-fpm.sock",
			"/run/php/php71-fpm.sock",
			"/run/php/php72-fpm.sock",
		},
		"tcp": {
			"localhost:9000",
			"127.0.0.1:9000",
		},
	}

	var client *fcgiclient.FCGIClient
	var err error

	for network, addresses := range commons {
		for _, address := range addresses {
			client, err = fcgiclient.DialTimeout(network, address, 100*time.Millisecond)
			if err == nil {
				config.ListenAddress = address
				config.ListenNetwork = network
				log.Println("Successfully connected to ", config.ListenNetwork, config.ListenAddress)
				client.Close()
				return nil
			}
		}
	}

	return nil
}

// findRunningBinary tries to list the processes on the machine and
// detect an *-fpm named process
func findRunningBinary() (*psutil.Process, error) {
	pp, _ := ps.Processes()

	for _, p := range pp {
		if strings.Contains(p.Executable(), "-fpm") {
			pid := p.Pid()
			proc, err := psutil.NewProcess(int32(pid))
			if err != nil {
				return nil, err
			}
			return proc, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

// parseCommandConfig sends a php-fpm -tt command line and parse the result
// to try to detect the listen URL and owner/groups issues
func parseCommandConfig(proc *psutil.Process, config *PhpFpmConfig) (map[string]string, error) {

	args, er := proc.CmdlineSlice()
	if er != nil {
		return nil, er
	}
	args = append(args, "-tt")
	cmd := exec.Command(args[0], args[1:]...)

	r, e := cmd.CombinedOutput()
	if e != nil {
		return nil, e
	}

	reg := regexp.MustCompile(`\[.*\] NOTICE:[ \t]*(.*)`)
	scanner := bufio.NewScanner(bytes.NewReader(r))

	configs := make(map[string]string)

	for scanner.Scan() {
		line := reg.ReplaceAllString(scanner.Text(), "${1}")
		if strings.Contains(line, " = ") {
			parts := strings.Split(line, " = ")
			key := strings.Trim(parts[0], " ")
			value := strings.Trim(parts[1], " ")
			if value != "undefined" {
				configs[key] = value
			}
		}
	}

	if listen, ok := configs["listen"]; ok {
		config.ListenAddress = listen
		if strings.Contains(config.ListenAddress, ":") {
			config.ListenNetwork = "tcp"
		} else {
			config.ListenAddress = "unix"
		}
	}
	if user, ok := configs["user"]; ok {
		config.PhpUser = user
	}
	if group, ok := configs["group"]; ok {
		config.PhpUser = group
	}
	if user, ok := configs["listen.owner"]; ok {
		config.ListenOwner = user
	}
	if group, ok := configs["listen.group"]; ok {
		config.ListenGroup = group
	}

	return configs, nil
}

// phpGetAsBytes sends a GET request to the FPM service pointing to the
// given php script. Returns the output as bytes
func phpGetAsBytes(script string, config *PhpFpmConfig) ([]byte, error) {

	env := make(map[string]string)
	env["SCRIPT_FILENAME"] = script
	env["SERVER_SOFTWARE"] = "go / fcgiclient "
	env["REMOTE_ADDR"] = "127.0.0.1"

	fcgi, err := fcgiclient.Dial(config.ListenNetwork, config.ListenAddress)
	if err != nil {
		return nil, err
	}

	resp, err := fcgi.Get(env)
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Println( "script", script, "content:", string(content))
	return content, nil

}
