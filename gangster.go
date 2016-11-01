package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
	"strings"
	"strconv"
	"regexp"
)

type Metric struct {
	Name  string `xml:"NAME,attr"`
	Value string `xml:"VAL,attr"`
}

type Host struct {
	Name     string   `xml:"NAME,attr"`
	Reported int      `xml:"REPORTED,attr"`
	Metrics  []Metric `xml:"METRIC"`
}

type Cluster struct {
	Name  string        `xml:"NAME,attr"`
	Hosts []Host        `xml:"HOST"`
}

type Ganglia struct {
	Clusters []Cluster `xml:"CLUSTER"`
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	if charset != "ISO-8859-1" {
		return nil, fmt.Errorf("unsupported charset %s\n", charset)
	}
	return input, nil
}

func check_env(env string, val string) {
	if len(val) != 0 {
		fmt.Printf("Using %s as %s;\n", val, env)
	} else {
		// in this case we have to exit
		fmt.Fprintf(os.Stderr, "Please specify %s\n", env)
		fmt.Printf("Please use '%s help' to get help\n", os.Args[0])
		os.Exit(1)
	}
}

func get_config() (string, string, string, string, string, string, string, string, string) {
	gmond_address := os.Getenv("GANGSTER_GMOND_ADDRESS")
	check_env("GANGSTER_GMOND_ADDRESS", gmond_address)
	gmond_port := os.Getenv("GANGSTER_GMOND_PORT")
	check_env("GANGSTER_GMOND_PORT", gmond_port)
	carbon_address := os.Getenv("GANGSTER_CARBON_ADDRESS")
	check_env("GANGSTER_CARBON_ADDRESS", carbon_address)
	carbon_port := os.Getenv("GANGSTER_CARBON_PORT")
	check_env("GANGSTER_CARBON_PORT", carbon_port)
	carbon_protocol := os.Getenv("GANGSTER_CARBON_PROTOCOL")
	check_env("GANGSTER_CARBON_PROTOCOL", carbon_protocol)
	graphite_prefix := os.Getenv("GANGSTER_GRAPHITE_PREFIX")
	cluster_as_a_prefix := os.Getenv("GANGSTER_CLUSTER_AS_A_PREFIX")
	log_file := os.Getenv("GANGSTER_LOG_FILE")
	sleep_time := os.Getenv("GANGSTER_SLEEP_TIME")
	return gmond_address, gmond_port, carbon_address, carbon_port, carbon_protocol, graphite_prefix, cluster_as_a_prefix, log_file, sleep_time
}

// returns data and success bool value
func get_gmond_xml() ([]byte, bool) {
	gmond_conn_string := gmond_address + ":" + gmond_port

	// dialing gmond...
	// connection timeout is 10 seconds
	conn_timeout := 10 * time.Second

	start_conn := time.Now()
	conn, err := net.DialTimeout("tcp", gmond_conn_string, conn_timeout)
	if err != nil {
		// we don't have to exit here, just keep trying
		log.Printf("Couldn't reach gmond at address %s\n", gmond_conn_string)
		return nil, false
	}

	gmond_date, err := ioutil.ReadAll(conn)
	if err != nil {
		// keep going
		log.Printf("Couldn't read gmond data!\n")
		return nil, false
	}
	log.Printf("Got ganglia data in %s\n", time.Since(start_conn))

	conn.Close()

	return gmond_date, true
}

func process_xml(gmond_data []byte) (Ganglia, error) {
	reader := bytes.NewReader(gmond_data)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charsetReader

	var xml_root Ganglia
	err := decoder.Decode(&xml_root)
	if err != nil {
		log.Printf("Couldn't unmarshal gmond XML!\n")
	}

	return xml_root, err
}

func connect_to_carbon() (net.Conn, error) {
	var carbon_ips []string
	// test if carbon address is ip
	carbon_ip := net.ParseIP(carbon_address)
	if carbon_ip.To4() != nil {
		carbon_ips = append(carbon_ips, carbon_address)
	} else {
		// or some DNS record
		var err error
		carbon_ips, err = net.LookupHost(carbon_address)
		if err != nil {
			log.Printf("Couldn't resolve %s\n", carbon_address)
		}
	}

	// build connection string
	conn_string := carbon_ips[0] + ":" + carbon_port

	// define connection timeout
	connection_timeout := 10 * time.Second

	// connect to carbon
	conn, err := net.DialTimeout(carbon_protocol, conn_string, connection_timeout)
	if err != nil {
		log.Printf("Can't connect to carbon at: %s", fmt.Sprintf("%s/%s", conn_string, carbon_protocol))
	}
	return conn, err
}

// returns nothing, fire and forget
func send_carbon_data(xml_root Ganglia, carbon_connection net.Conn) {
	for _, cluster := range xml_root.Clusters {
		if cluster_as_a_prefix != "" {
			graphite_prefix = cluster.Name + "."
		}

		for _, host := range cluster.Hosts {
			hostname := host.Name
			reported := host.Reported
			for _, metric := range host.Metrics {
				metric_string := fmt.Sprintf("%s%s.%s.sum %s %d\n", graphite_prefix, hostname, strings.Replace(metric.Name, " ", "_", -1), metric.Value, reported)
				fmt.Fprintf(carbon_connection, metric_string)
			}
		}
		log.Printf("Sent a bunch of metrics for cluster %s", cluster.Name)

	}

	carbon_connection.Close()
}

var gmond_address, gmond_port, carbon_address, carbon_port, carbon_protocol, graphite_prefix, log_file, cluster_as_a_prefix, sleep_time string

// sleep time when connect to gmond fails
var ERR_INIT_SLEEP_TIME = 2
var ERR_MAX_SLEEP_TIM = 32

// error sleep time multiplier
var ERR_SLEEP_TIME_MX = 2

// sleep between polls
var DEFAULT_SLEEP_TIME = 0

func main() {
	// some kind of help
	if len(os.Args) > 1 {
		if os.Args[1] == "help" {
			fmt.Fprintf(os.Stdout, "%s looks for following environment variables:\n", os.Args[0])
			fmt.Fprintf(os.Stdout, "GANGSTER_GMOND_ADDRESS [mandatory]: address which gmond listens. Exmaple: 127.0.0.1\n")
			fmt.Fprintf(os.Stdout, "GANGSTER_GMOND_PORT [mandatory]: port which listens gmond. Example: 8649\n")
			fmt.Fprintf(os.Stdout, "GANGSTER_CARBON_ADDRESS [mandatory]: address where %s should send metrics. Example: carbon01\n", os.Args[0])
			fmt.Fprintf(os.Stdout, "GANGSTER_CARBON_PORT [mandatory]: port where %s should send metrics. Example: 2003\n", os.Args[0])
			fmt.Fprintf(os.Stdout, "GANGSTER_CARBON_PROTOCOL [mandatory]: protocol which %s should use for sending. Example: udp\n", os.Args[0])
			fmt.Fprintf(os.Stdout, "GANGSTER_GRAPHITE_PREFIX: prefix for metrics. Example: zone.mgmt.\n")
			fmt.Fprintf(os.Stdout, "GANGSTER_LOG_FILE: log file location, Example: /mnt/log_file\n")
			fmt.Fprintf(os.Stdout, "GANGSTER_CLUSTER_AS_A_PREFIX: use Ganglia cluster as a prefix for graphite metric\n")
			fmt.Fprintf(os.Stdout, "GANGSTER_SLEEP_TIME: sleep time between gmond polls\n")
		} else {
			fmt.Fprintf(os.Stderr, "Use '%s help' for help\n", os.Args[0])
		}
		os.Exit(0)
	}

	gmond_address, gmond_port, carbon_address, carbon_port, carbon_protocol, graphite_prefix, cluster_as_a_prefix, log_file, sleep_time = get_config()

	if log_file != "" {
		file, err := os.OpenFile(log_file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			// log file was specified so we should care about it
			fmt.Fprintf(os.Stderr, "Couldn't open %s!\n", log_file)
			panic(err)
		}
		log.SetOutput(file)
		defer file.Close()
	} else {
		log.SetOutput(os.Stdout)
	}

	rand.Seed(time.Now().Unix())

	// sleep interval
	// error sleep interval
	err_sleep_time := ERR_INIT_SLEEP_TIME
	var sleep_duration time.Duration = time.Duration(DEFAULT_SLEEP_TIME)
	is_positive_int, _ := regexp.MatchString("^[0-9]+$", sleep_time)
	if is_positive_int == true {
		sleep_time_int, err := strconv.Atoi(sleep_time)
		if err != nil {
			log.Printf("Can't convert sleep time to int. Using default sleep time\n")
		} else {
			sleep_duration = time.Duration(sleep_time_int)
		}
	}

	for {
		gmond_xml, is_success := get_gmond_xml()
		if is_success == false {
			log.Printf("Something bad happened when talked to gmond!\n")
			log.Printf("Sleeping %d seconds!\n", err_sleep_time)
			time.Sleep(time.Duration(err_sleep_time) * time.Second)
			err_sleep_time = err_sleep_time * ERR_SLEEP_TIME_MX
			if err_sleep_time > ERR_MAX_SLEEP_TIM {
				// flush sleep time
				err_sleep_time = ERR_INIT_SLEEP_TIME
			}
			continue
		}

		log.Printf("Start ganglia data processing...")

		ganglia_data, err := process_xml(gmond_xml)
		if err != nil {
			continue
		}
		log.Printf("Ganglia data processing has finished!")

		// send data to carbon
		carbon_conn, err := connect_to_carbon()
		if err != nil {
			continue
		}
		send_carbon_data(ganglia_data, carbon_conn)

		// init err_sleep_time again
		err_sleep_time = ERR_INIT_SLEEP_TIME
		// and sleep before the next turn
		time.Sleep(sleep_duration * time.Second)
	}
}
