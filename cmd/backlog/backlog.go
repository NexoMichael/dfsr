package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"gopkg.in/adsi.v0"
	"gopkg.in/dfsr.v0/config"
	"gopkg.in/dfsr.v0/helper"
)

var domainFlag string
var groupFlag regexSlice
var fromFlag regexSlice
var toFlag regexSlice
var memberFlag regexSlice
var loopFlag uintOrInf
var delaySecondsFlag uint
var cacheSecondsFlag uint
var skipFlag regexSlice
var minFlag uint
var verboseFlag bool

const defaultLoopValue = 1

func init() {
	flag.StringVar(&domainFlag, "d", "", "domain to query")
	flag.Var(&groupFlag, "g", "group to query")
	flag.Var(&fromFlag, "f", "regex of source hostname")
	flag.Var(&toFlag, "t", "regex of dest hostname")
	flag.Var(&memberFlag, "m", "regex of member hostname (matches either dest or source)")
	flag.Var(&loopFlag, "loop", "number of iterations or \"infinite\"")
	flag.UintVar(&delaySecondsFlag, "delay", 5, "number of seconds to deley between loops")
	flag.UintVar(&cacheSecondsFlag, "cache", 5, "number of seconds to cache vectors")
	flag.Var(&skipFlag, "skip", "regex of hostname to skip")
	flag.UintVar(&minFlag, "min", 0, "minimum backlog to display")
	flag.BoolVar(&verboseFlag, "v", false, "verbose")
}

func main() {
	flag.Parse()

	if !loopFlag.Present {
		loopFlag.Value = defaultLoopValue
	}

	if !loopFlag.Inf && loopFlag.Value == 0 {
		return
	}

	domain, connections, err := setup(domainFlag, groupFlag, fromFlag, toFlag, memberFlag, skipFlag)
	if err != nil {
		log.Fatal(err)
	}

	client, err := helper.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	if cacheSecondsFlag > 0 {
		client.Cache(time.Duration(cacheSecondsFlag) * time.Second)
	}

	if loopFlag.Inf {
		for loop := uint(0); ; loop++ {
			run(domain, loop, minFlag, client, connections)
			time.Sleep(time.Duration(delaySecondsFlag) * time.Second)
		}
	} else {
		for loop := uint(0); loop < loopFlag.Value; loop++ {
			run(domain, loop, minFlag, client, connections)
			if loop+1 < loopFlag.Value {
				fmt.Println("")
				time.Sleep(time.Duration(delaySecondsFlag) * time.Second)
			}
		}
	}
}

func run(domain string, iteration uint, min uint, client *helper.Client, connections []connection) {
	var wg sync.WaitGroup
	wg.Add(len(connections))

	//fmt.Printf("[query %v] %s\n", iteration, domain)
	fmt.Printf("%-50s %-50s %-50s %-15s %s\n", "Group", "Source", "Destination", "Backlog", "Time")
	fmt.Printf("%-50s %-50s %-50s %-15s %s\n", "-----", "------", "-----------", "-------", "----")

	start := time.Now()

	for i := 0; i < len(connections); i++ {
		go connections[i].ComputeBacklog(client, &wg)
	}

	wg.Wait()

	finish := time.Now()

	for i := 0; i < len(connections); i++ {
		c := &connections[i]

		if c.TotalBacklog() < min {
			continue
		}

		fmt.Printf("%-50s %-50s %-50s ", c.Group.Name, c.From, c.To)
		if c.Err != nil {
			fmt.Printf("%-15v ", c.Err)
			//fmt.Printf("%-15v ", c.Call)
		} else {
			fmt.Printf("%-15s ", fmt.Sprint(c.Backlog))
		}
		fmt.Printf("%v\n", c.Call.Duration())
		if verboseFlag {
			fmt.Printf("Call: %v\n", c.Call)
		}
	}

	fmt.Printf("Total Time: %v\n", finish.Sub(start))
}

func setup(domain string, groupRegex, fromRegex, toRegex, memberRegex, skipRegex regexSlice) (dom string, connections []connection, err error) {
	client, err := adsi.NewClient()
	if err != nil {
		return "", nil, err
	}
	defer client.Close()

	if domain == "" {
		domain, err = dnc(client)
		if err != nil {
			return "", nil, err
		}
	}
	dom = domain

	d, err := config.Domain(client, domain)
	if err != nil {
		return domain, nil, err
	}

	for g := 0; g < len(d.Groups); g++ {
		group := &d.Groups[g]
		if !isMatch(group.Name, groupRegex, true) {
			continue
		}

		for m := 0; m < len(group.Members); m++ {
			member := &group.Members[m]
			to := member.Computer.Host
			if to == "" {
				continue
			}
			if isMatch(to, skipRegex, false) {
				continue
			}
			if !isMatch(to, toRegex, true) {
				continue
			}

			for c := 0; c < len(member.Connections); c++ {
				conn := &member.Connections[c]
				from := conn.Computer.Host
				if from == "" {
					continue
				}
				if !conn.Enabled {
					continue
				}
				if isMatch(from, skipRegex, false) {
					continue
				}
				if !isMatch(from, fromRegex, true) {
					continue
				}
				if !isMatch(from, memberRegex, true) && !isMatch(to, memberRegex, true) {
					continue
				}

				connections = append(connections, connection{
					From:  from,
					To:    to,
					Group: group,
				})
			}
		}
	}
	return
}
